package repository_after

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sort"
)

var (
	ErrInvalidHeader = errors.New("invalid header")
)

// writeHeader writes a simple header: magic(4) | padding(1) | count(uint16) | entries(value:uint8,freq:uint64...)
func writeHeader(w io.Writer, freqs map[byte]uint64, padding uint8) error {
	bw := bufio.NewWriter(w)
	if _, err := bw.Write([]byte("HUF1")); err != nil {
		return err
	}
	if err := bw.WriteByte(padding); err != nil {
		return err
	}
	// deterministic order
	keys := make([]int, 0, len(freqs))
	for k := range freqs {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	if err := binary.Write(bw, binary.LittleEndian, uint16(len(keys))); err != nil {
		return err
	}
	for _, ki := range keys {
		b := byte(ki)
		if err := bw.WriteByte(b); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.LittleEndian, freqs[b]); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// readHeader reads header from a bufio.Reader and returns freqs map and padding
func readHeader(br *bufio.Reader) (map[byte]uint64, uint8, error) {
	magic := make([]byte, 4)
	if _, err := io.ReadFull(br, magic); err != nil {
		return nil, 0, err
	}
	if string(magic) != "HUF1" {
		return nil, 0, ErrInvalidHeader
	}
	p, err := br.ReadByte()
	if err != nil {
		return nil, 0, err
	}
	var count uint16
	if err := binary.Read(br, binary.LittleEndian, &count); err != nil {
		return nil, 0, err
	}
	freqs := make(map[byte]uint64, count)
	for i := 0; i < int(count); i++ {
		b, err := br.ReadByte()
		if err != nil {
			return nil, 0, err
		}
		var f uint64
		if err := binary.Read(br, binary.LittleEndian, &f); err != nil {
			return nil, 0, err
		}
		freqs[b] = f
	}
	return freqs, p, nil
}

// Encode enforces 2-pass streaming using a temporary file to avoid RAM issues.
func Encode(r io.Reader, w io.Writer) error {
	// Create temp file for the second pass
	tmp, err := os.CreateTemp("", "huff_encode_*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	// 1st pass: Analyze frequencies and buffer input to disk
	var freqsArray [256]uint64
	buf := make([]byte, 64*1024)
	var totalBytes int64

	for {
		n, err := r.Read(buf)
		if n > 0 {
			for i := 0; i < n; i++ {
				freqsArray[buf[i]]++
			}
			if _, wErr := tmp.Write(buf[:n]); wErr != nil {
				return wErr
			}
			totalBytes += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if totalBytes == 0 {
		return writeHeader(w, make(map[byte]uint64), 0)
	}

	// Convert array back to map for existing functions
	freqs := make(map[byte]uint64)
	for i, count := range freqsArray {
		if count > 0 {
			freqs[byte(i)] = count
		}
	}

	root := buildTree(freqs)
	if root.left == nil && root.right == nil {
		return writeHeader(w, freqs, 0)
	}
	
	codes := generateCodes(root)

	// Pre-map codes to a slice for faster lookup
	type codeInfo struct {
		val uint64
		len int
	}
	var codesLookup [256]codeInfo
	var totalBits int64
	for b, c := range codes {
		codesLookup[b] = codeInfo{val: c.Val, len: c.Len}
		totalBits += int64(freqs[b]) * int64(c.Len)
	}
	
	padding := uint8((8 - (totalBits % 8)) % 8)
	if err := writeHeader(w, freqs, padding); err != nil {
		return err
	}

	if _, err := tmp.Seek(0, 0); err != nil {
		return err
	}

	bw := bufio.NewWriterSize(w, 64*1024)
	var accumulator uint64
	var bitsInAccumulator int
	
	br := bufio.NewReaderSize(tmp, 64*1024)
	readBuf := make([]byte, 64*1024)
	for {
		n, err := br.Read(readBuf)
		if n > 0 {
			for i := 0; i < n; i++ {
				info := codesLookup[readBuf[i]]
				accumulator = (accumulator << info.len) | info.val
				bitsInAccumulator += info.len

				for bitsInAccumulator >= 8 {
					bitsInAccumulator -= 8
					if err := bw.WriteByte(byte(accumulator >> bitsInAccumulator)); err != nil {
						return err
					}
				}
				accumulator &= (1 << bitsInAccumulator) - 1
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if bitsInAccumulator > 0 {
		accumulator <<= (8 - bitsInAccumulator)
		if err := bw.WriteByte(byte(accumulator)); err != nil {
			return err
		}
	}

	return bw.Flush()
}

// Decode reads header and payload streams.
func Decode(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	freqs, _, err := readHeader(br)
	if err != nil {
		return err
	}
	if len(freqs) == 0 {
		return nil
	}

	var expected uint64
	for _, f := range freqs {
		expected += f
	}

	root := buildTree(freqs)
	
	// Single symbol optimization
	if root.left == nil && root.right == nil {
		if root.value < 0 || root.value > 255 {
			return errors.New("invalid leaf value")
		}
		b := byte(root.value)
		bw := bufio.NewWriter(w)
		for i := uint64(0); i < expected; i++ {
			if err := bw.WriteByte(b); err != nil {
				return err
			}
		}
		return bw.Flush()
	}

	// Decode stream
	bw := bufio.NewWriter(w)
	cur := root
	var decodedCount uint64
	
	// Read byte by byte
	for decodedCount < expected {
		b, err := br.ReadByte()
		if err == io.EOF {
			break 
		}
		if err != nil {
			return err
		}

		// Process 8 bits (unless near end and padding matters)
		// Actually, we can just process until we hit expected count.
		// Padding bits at the very end will just be ignored if we stop at expected count.
		
		// To be strictly correct with padding, we should know total bits, but we rely on 'expected' count.
		// If we hit EOF before expected, it's an error.
		
		for i := 7; i >= 0; i-- {
			bit := (b >> i) & 1
			if bit == 0 {
				cur = cur.left
			} else {
				cur = cur.right
			}
			
			if cur == nil {
				return errors.New("invalid bitstream")
			}

			if cur.left == nil && cur.right == nil {
				// leaf
				if err := bw.WriteByte(byte(cur.value)); err != nil {
					return err
				}
				decodedCount++
				cur = root
				
				if decodedCount == expected {
					// Done, ignore remaining padding bits in this byte
					break
				}
			}
		}
	}

	if decodedCount != expected {
		return errors.New("unexpected EOF or data length mismatch")
	}

	return bw.Flush()
}
