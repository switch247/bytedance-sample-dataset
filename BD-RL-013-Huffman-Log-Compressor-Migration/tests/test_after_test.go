package repository_after_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"math/rand"
	"testing"
	"time"

	repo "migration-demo/repository_after"
)

func parseHeader(b []byte) (map[byte]uint64, uint8, error) {
	br := bufio.NewReader(bytes.NewReader(b))
	magic := make([]byte, 4)
	if _, err := io.ReadFull(br, magic); err != nil {
		return nil, 0, err
	}
	if string(magic) != "HUF1" {
		return nil, 0, repo.ErrInvalidHeader
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

// local deterministic tree builder and code generator for tests
type tNode struct {
	val   int
	freq  uint64
	left  *tNode
	right *tNode
}

// simple O(n^2) deterministic build using sorted keys and merging
func buildTreeFromFreqs(freqs map[byte]uint64) *tNode {
	keys := make([]int, 0, len(freqs))
	for k := range freqs {
		keys = append(keys, int(k))
	}
	// sort keys
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	nodes := make([]*tNode, 0, len(keys))
	for _, k := range keys {
		b := byte(k)
		nodes = append(nodes, &tNode{val: int(b), freq: freqs[b]})
	}
	if len(nodes) == 0 {
		return nil
	}
	for len(nodes) > 1 {
		// take two smallest (nodes sorted by freq then val)
		// simple linear scan
		min1, min2 := 0, 1
		if nodes[min2].freq < nodes[min1].freq || (nodes[min2].freq == nodes[min1].freq && nodes[min2].val < nodes[min1].val) {
			min1, min2 = min2, min1
		}
		for i := 2; i < len(nodes); i++ {
			if nodes[i].freq < nodes[min1].freq || (nodes[i].freq == nodes[min1].freq && nodes[i].val < nodes[min1].val) {
				min2 = min1
				min1 = i
			} else if nodes[i].freq < nodes[min2].freq || (nodes[i].freq == nodes[min2].freq && nodes[i].val < nodes[min2].val) {
				min2 = i
			}
		}
		a := nodes[min1]
		b := nodes[min2]
		if min1 < min2 {
			// remove min2 then min1
			nodes = append(nodes[:min2], nodes[min2+1:]...)
			nodes = append(nodes[:min1], nodes[min1+1:]...)
		} else {
			nodes = append(nodes[:min1], nodes[min1+1:]...)
			nodes = append(nodes[:min2], nodes[min2+1:]...)
		}
		merged := &tNode{val: -1, freq: a.freq + b.freq, left: a, right: b}
		// insert merged keeping deterministic order: append then stable insert by freq
		nodes = append(nodes, merged)
	}
	return nodes[0]
}

func generateCodesFromTree(root *tNode) map[byte]string {
	codes := make(map[byte]string)
	if root == nil {
		return codes
	}
	var walk func(n *tNode, prefix string)
	walk = func(n *tNode, prefix string) {
		if n == nil {
			return
		}
		if n.left == nil && n.right == nil && n.val >= 0 {
			if prefix == "" {
				codes[byte(n.val)] = "0"
			} else {
				codes[byte(n.val)] = prefix
			}
			return
		}
		walk(n.left, prefix+"0")
		walk(n.right, prefix+"1")
	}
	walk(root, "")
	return codes
}

func TestRoundtripRandom10KB(t *testing.T) {
	data := make([]byte, 10*1024)
	_, _ = rand.Read(data)

	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !bytes.Equal(dec.Bytes(), data) {
		t.Fatalf("roundtrip mismatch: got %d bytes", dec.Len())
	}
}

func TestEmptyRoundtrip(t *testing.T) {
	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader([]byte{}), &enc); err != nil {
		t.Fatalf("Encode empty failed: %v", err)
	}
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatalf("Decode empty failed: %v", err)
	}
	if dec.Len() != 0 {
		t.Fatalf("expected empty output, got %d bytes", dec.Len())
	}
}

func TestRoundtripAllBytesOnce(t *testing.T) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !bytes.Equal(dec.Bytes(), data) {
		t.Fatalf("roundtrip mismatch for all-bytes input: got %d bytes", dec.Len())
	}
}

func TestSingleByteRepeated(t *testing.T) {
	data := bytes.Repeat([]byte{'A'}, 1024*1024)
	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !bytes.Equal(dec.Bytes(), data) {
		t.Fatalf("roundtrip mismatch for single-byte repeated: got %d bytes", dec.Len())
	}
}

func TestSmallInputs(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		data := make([]byte, n)
		_, _ = rand.Read(data)
		var enc bytes.Buffer
		if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		var dec bytes.Buffer
		if err := repo.Decode(bytes.NewReader(enc.Bytes()), &dec); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}
		if !bytes.Equal(dec.Bytes(), data) {
			t.Fatalf("roundtrip mismatch for small input n=%d", n)
		}
	}
}

func TestHeaderPaddingParity(t *testing.T) {
	data := make([]byte, 123)
	_, _ = rand.Read(data)
	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	freqs, padding, err := parseHeader(enc.Bytes())
	if err != nil {
		t.Fatalf("parseHeader failed: %v", err)
	}
	var sum uint64
	for _, f := range freqs {
		sum += f
	}
	if int(sum) != len(data) {
		t.Fatalf("freqs sum %d != data len %d", sum, len(data))
	}
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if dec.Len() != len(data) {
		t.Fatalf("decoded length %d != original %d (padding %d)", dec.Len(), len(data), padding)
	}
}

func TestHeaderDeterministic(t *testing.T) {
	data := []byte("deterministic-test-abc123")
	var a bytes.Buffer
	var b bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &a); err != nil {
		t.Fatalf("Encode a failed: %v", err)
	}
	if err := repo.Encode(bytes.NewReader(data), &b); err != nil {
		t.Fatalf("Encode b failed: %v", err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("encodings differ on same input")
	}
}

func TestTruncatedStream(t *testing.T) {
	data := make([]byte, 2048)
	_, _ = rand.Read(data)
	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	full := enc.Bytes()
	if len(full) < 10 {
		t.Fatalf("encoded too small")
	}
	truncated := full[:len(full)-3]
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(truncated), &dec); err == nil {
		t.Fatalf("expected error on truncated stream, got nil")
	}
}

func TestCorruptedHeader(t *testing.T) {
	data := make([]byte, 512)
	_, _ = rand.Read(data)
	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	out := enc.Bytes()
	out[0] ^= 0xFF
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(out), &dec); err == nil {
		t.Fatalf("expected header error on corrupted header, got nil")
	}
}

func TestCodesConsistency(t *testing.T) {
	data := append(bytes.Repeat([]byte{'A'}, 100), bytes.Repeat([]byte{'B'}, 10)...)
	data = append(data, bytes.Repeat([]byte{'C'}, 2)...)
	freqs := make(map[byte]uint64)
	for _, b := range data {
		freqs[b]++
	}
	root := buildTreeFromFreqs(freqs)
	codes := generateCodesFromTree(root)
	if len(codes[byte('A')]) > len(codes[byte('B')]) {
		t.Fatalf("expected A code length <= B code length")
	}
}

func TestStreamingSmallBuffer(t *testing.T) {
	data := make([]byte, 1024*16)
	_, _ = rand.Read(data)
	r := &chunkReader{data: data, chunkSize: 64}
	var enc bytes.Buffer
	if err := repo.Encode(r, &enc); err != nil {
		t.Fatalf("Encode streaming failed: %v", err)
	}
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatalf("Decode streaming failed: %v", err)
	}
	if !bytes.Equal(dec.Bytes(), data) {
		t.Fatalf("streaming roundtrip mismatch")
	}
}

type chunkReader struct {
	data      []byte
	pos       int
	chunkSize int
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if c.pos >= len(c.data) {
		return 0, io.EOF
	}
	n := c.chunkSize
	if remaining := len(c.data) - c.pos; remaining < n {
		n = remaining
	}
	copy(p, c.data[c.pos:c.pos+n])
	c.pos += n
	return n, nil
}
func TestPerformance50MB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}
	size := 50 * 1024 * 1024
	data := make([]byte, size)
	_, _ = rand.Read(data)

	// Encode
	start := time.Now()
	var enc bytes.Buffer
	if err := repo.Encode(bytes.NewReader(data), &enc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	encDur := time.Since(start)

	// Decode
	encoded := enc.Bytes()
	start = time.Now()
	var dec bytes.Buffer
	if err := repo.Decode(bytes.NewReader(encoded), &dec); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	decDur := time.Since(start)

	if !bytes.Equal(dec.Bytes(), data) {
		t.Fatalf("Performance mismatch")
	}

	t.Logf("Encode 50MB: %v, Decode 50MB: %v", encDur, decDur)
	if encDur > 5*time.Second {
		t.Errorf("Encode too slow: %v > 5s", encDur)
	}
}

// Failing reader for testing errors
type failReader struct {
	err error
}

func (f *failReader) Read(p []byte) (n int, err error) {
	return 0, f.err
}

// Large stream simulation that generates predictable pattern without allocation
type largePatternReader struct {
	size int64
	read int64
}

func (r *largePatternReader) Read(p []byte) (n int, err error) {
	if r.read >= r.size {
		return 0, io.EOF
	}
	remaining := r.size - r.read
	if int64(len(p)) > remaining {
		n = int(remaining)
	} else {
		n = len(p)
	}
	// fill with pattern based on position
	for i := 0; i < n; i++ {
		p[i] = byte((r.read + int64(i)) & 0xFF)
	}
	r.read += int64(n)
	return n, nil
}

func TestStreamingLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large streaming test")
	}
	// Test 100MB stream processing without full memory load
	size := int64(100 * 1024 * 1024)
	r := &largePatternReader{size: size}
	
	// We just want to ensure it doesn't crash or error out. 
	// To fully verify memory, we'd need profiling, but valid execution is a good proxy.
	// We'll discard output to avoid OOM on the writer side if we buffered it all.
	// But `repo.Encode` writes to a writer.
	
	// Use a discarding writer that counts bytes to verify output is produced
	cw := &countingWriter{}
	if err := repo.Encode(r, cw); err != nil {
		t.Fatalf("Encode large stream failed: %v", err)
	}
	
	if cw.n == 0 {
		t.Fatalf("No output produced for large stream")
	}
	t.Logf("Processed %d MB stream, output %d bytes", size/1024/1024, cw.n)
}

type countingWriter struct {
	n int64
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	cw.n += int64(len(p))
	return len(p), nil
}
