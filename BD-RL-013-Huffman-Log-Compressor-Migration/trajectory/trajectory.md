# Migration Documentation: Huffman Compressor (Python → Go)

## Overview

This document records the migration of a Huffman compression utility from a Python implementation to a Go implementation. It captures our understanding, design questions, what currently works, goals for improvement, a multi-step migration plan with explanations for each step, verification testcases and rationale, a complete chain-of-thought for the implementation (no missing steps), and curated external references.

## Understanding

- **Starting State**: The project initially contained only the `repository_before` directory (a legacy Python implementation). No Go code was present at the onset.
- **Goal**: Refactor/Migrate to a Go implementation that solves the memory and performance bottlenecks of the Python string-based approach.
- **Requirements summarized**:
  - Use bitwise operators for packing/unpacking; avoid string bit buffers.
  - Support streaming via `io.Reader` and `io.Writer`.
  - Include a serialized header (tree or frequency table) for cross-language compatibility.
  - Store padding metadata so the decoder can ignore trailing filler bits.
  - Robust error handling for corrupted headers/truncated streams/empty input.
  - Handle all 256 byte values and single-symbol inputs.
  - Provide unit tests as the **final verification step**, including a 10KB random roundtrip and adversarial truncated/header-corrupted tests.

## Questions to Understand (for stakeholders / future readers)

- Do we require binary compatibility with a specific Python header format, or may we choose a deterministic header format as long as Python and Go agree? (We implemented a deterministic header `HUF1` + padding + frequency table.)
- Is two-pass encoding (scan to collect frequencies, then encode) acceptable, or do we need a single-pass streaming encoder? (Two-pass is typical for Huffman; single-pass requires external frequency metadata or adaptive Huffman.)
- Are there constraints on header size or how compact the frequency table must be serialized? (We used a compact deterministic ordering with uint64 frequencies.)
- What are the operational performance targets for the Go implementation (throughput, memory ceiling)? (README suggests >50MB/s; exact target may vary by infra.)
- Should the Go decoder accept compressed streams created by unknown third-party producers, or only our Python predecessor? (We prioritized parity with the provided Python format.)

## What Is Working (Current State)

- A Go package implementing Huffman tree construction, deterministic priority queue ordering, code generation, bitwise packing, and header serialization.
- Public API functions `Encode(io.Reader, io.Writer)` and `Decode(io.Reader, io.Writer)` that accept streams.
- Header serialization format implemented: 4-byte magic `HUF1`, 1-byte padding count, 2-byte entry count (uint16 little-endian), followed by entries of 1-byte symbol + 8-byte uint64 frequency (little-endian). Deterministic ordering ensures repeatable encodings.
- Unit tests added under `/tests` that exercise: random 10KB roundtrip, empty input, all-bytes input, single-byte repeated, small-inputs, header parity, deterministic encodings, truncated stream, corrupted header, and a streaming small-chunk reader test.
- Tests run via `go test ./...` and passed locally for the moved test file.

## What We Need to Improve (Goals)

- Streaming memory usage: Current `Encode` reads the source to compute frequencies then encodes in-memory; for very large inputs we should support streaming encoders with either pre-supplied frequency maps or a two-pass approach that reads input in chunks and writes intermediate frequency metadata to disk or another stream. If strict single-pass is required, we must adopt an adaptive Huffman algorithm, which is more complex.
- Performance benchmarking: Run large-sample comparisons (e.g., 50MB+ logs) against the Python baseline and profile hotspots to ensure throughput targets (e.g., >50MB/s) and acceptable CPU/memory usage.
- Header compactness: If header size is a concern, we can compress the frequency table (e.g., write only non-zero entries with variable-length integer encodings) or serialize a canonical tree shape instead of full 8-byte frequencies.
- Cross-language test artifacts: Provide Python ↔ Go roundtrip test vectors (small sample files) to validate interoperability in CI and production.

## Migration Implementations (multi-step, with explanations)

1. Preparation: Create test cases and define header format
   - Why: Tests drive development and ensure correctness across language boundaries.
   - Actions: Draft `TEST_CASES.md` and add unit tests targeted at required behaviors (roundtrip, edge cases, corrupted/truncated streams).

2. Implement deterministic Huffman core in Go (`huffman_core.go`)
   - Why: The core data structures (node, heap) must be deterministic to produce repeatable headers across runs and languages.
   - Actions:
     - Define `Node` struct: `value byte | freq uint64 | left/right *Node`.
     - Implement a deterministic priority queue using `container/heap` with tie-breakers (lower byte value wins on equal frequencies).
     - Build the Huffman tree from a frequency map (map[byte]uint64).
     - Generate codes by traversing the tree (assign `0` to left, `1` to right); produce canonical codes for deterministic serialization.

3. Implement streaming-safe Encode/Decode API (`huffman_service.go`)
   - Why: Expose `Encode(io.Reader, io.Writer)` and `Decode(io.Reader, io.Writer)` for large-file support.
   - Actions for `Encode`:
     - First pass: read from `io.Reader` to compute frequencies. (Note: this requires either buffering or reading the source twice; for files this is acceptable by rewinding or reading from disk.)
     - Build tree and generate codes.
     - Serialize header: magic, padding placeholder, count, then symbol-frequency pairs in deterministic order.
     - Second pass: re-read input (or iterate over stored input chunks) and write encoded bits by accumulating bits into a byte buffer using bit shifts and OR operations. Track final padding and update header padding byte accordingly.
   - Actions for `Decode`:
     - Read and validate header, reconstruct frequency map and tree.
     - Decode bitstream by reading bytes, unpacking bits MSB-first using shifts and masks, walking the Huffman tree per bit and writing output bytes to `io.Writer`.
     - Use the padding metadata to ignore trailing padding bits in the final byte.

4. Edge-case and error handling
   - Why: Robustness in production requires clear errors for corrupted/truncated data.
   - Actions:
     - Return descriptive errors for invalid magic, unexpected EOF while reading header entries, and decoding bitstream inconsistencies (e.g., decoded symbol count not matching frequencies).
     - Support single-symbol alphabet: encode header and avoid writing payload bits (decoder repeats symbol according to frequency).

5. Tests and verification harness
   - Why: Validate correctness, parity, and performance.
   - Actions:
     - Implement unit tests for all functional requirements (roundtrip, empty, single-symbol, all-bytes, truncated, corrupted header).
     - Add a 10KB random roundtrip test specifically required.
     - Add a 50MB benchmark comparison test harness that runs the Go implementation and measures time vs a Python baseline (the Python run can be measured externally or with `time`), capturing throughput and memory.

6. Performance tuning and optional streaming-only encoder
   - Why: To meet strict memory and throughput targets.
   - Actions:
     - Profile encoder/decoder, optimize critical loops (bit packing/unpacking), reuse buffers, and avoid allocations in hot paths.
     - If required, implement adaptive Huffman or an external frequency pre-pass that emits frequency metadata separate from the payload to allow a single-pass streaming encoder.

7. Documentation and release
   - Why: Ensure maintainability and reproducibility.
   - Actions:
     - Add README with usage examples for streaming CLI and library use.
     - Produce small sample vectors (Python-compressed files and Go-decompressed verifications) for CI.

## Verification Testcases — design and rationale

Test selection principles:
- Cover typical, edge, and adversarial cases.
- Ensure cross-language parity and deterministic behavior.
- Validate error conditions explicitly.

Key testcases implemented and why they verify requirements:

- Random 10KB Roundtrip (unit):
  - Purpose: Functional correctness and reproducibility on arbitrary data; required by assignment.
  - Rationale: Random input exercises the code generation path, non-trivial symbol distributions, and ensures full encode/decode symmetry.

- Empty Input:
  - Purpose: Ensure encoder/decoder handle zero-length input gracefully.
  - Rationale: Edge-case; verifies no header corruption or erroneous writes.

- Single-Byte Repeated (large):
  - Purpose: Ensure single-symbol alphabet handling.
  - Rationale: Huffman tree degenerates to a single node; encoder/decoder must not rely on two-node tree assumptions.

- All 256 Bytes Once:
  - Purpose: Validate support for full byte alphabet and header encoding of all symbols.
  - Rationale: Ensures serialization/deserialization of frequency table and proper code generation.

- Small Inputs (1-3 bytes):
  - Purpose: Validate tiny inputs and code assignment when code lengths may be minimal.

- Header Padding Parity:
  - Purpose: Verify the header padding value matches the encoded payload so decoder ignores trailing bits.

- Deterministic Encodings:
  - Purpose: Confirm same input leads to identical encodings (important for reproducible test vectors and cross-language parity).

- Truncated Stream (adversarial):
  - Purpose: Ensure decoder returns a specific/diagnosable error on truncated bitstreams.

- Corrupted Header (adversarial):
  - Purpose: Ensure decoder rejects invalid magic or malformed header.

- Streaming Small-Chunk Read:
  - Purpose: Validate `Encode` and `Decode` behave correctly when `io.Reader` provides small chunks (real-world streaming).

How testcases were chosen:
- Started from the assignment's explicit test requirements (10KB random, 50MB performance, adversarial tests).
- Added edge cases common to Huffman implementations (empty, single-symbol, all-bytes) to validate correctness across the domain.
- Added deterministic and header parity tests to ensure cross-language compatibility and reproducible outputs.

## Chain-of-thought (complete implementation reasoning)

1. Requirement gathering and constraints
   - The encoder must use bitwise ops and support streaming. Huffman coding requires knowledge of symbol frequencies to build an optimal tree; therefore a frequency pass is necessary unless an adaptive algorithm is chosen.

2. Architecture for Scaling (The "Streaming" Thought)
   - To process files larger than RAM, we cannot use `io.ReadAll`. We decided on a **2-pass streaming approach**.
   - **Pass 1**: Read the input stream in chunks (buffered) to calculate character frequencies.
   - **Internal Buffering**: To support generic non-seekable `io.Reader` inputs, we buffer Pass 1 data to a **temporary disk file** (`os.CreateTemp`). This ensures memory usage remains constant (O(1) relative to file size).
   - **Pass 2**: Rewind the temporary file and perform the actual bitwise encoding.

3. Optimization for Performance (The "Bitwise" Thought)
   - String concatenation of bits (`"0" + "1"`) is the primary bottleneck in the legacy code.
   - **Accumulator Pattern**: We use a `uint64` accumulator to pack bits. For each byte, we fetch its bit-code and shift it into the accumulator.
   - **Batch Flushing**: Once the accumulator holds 8 or more bits, we flush those bytes to the output buffer and retain the remainder. This minimizes bit-level loops and leverages the CPU's ability to process words.
   - **Fixed-Array Lookups**: Instead of map lookups (which incur hashing overhead), we use a `[256]Code` array for O(1) code retrieval during the final encoding loop.

4. Decide header format
   - Choose a compact deterministic header that is unambiguous and easy to parse: fixed magic `HUF1` + padding byte + entry count + symbol/frequency pairs.
   - Use little-endian binary encoding for counts to simplify `encoding/binary` usage.

5. Deterministic ordering
   - To ensure identical encodings across runs, sort symbol entries deterministically. Use frequency first, then symbol value as tie-breaker.

6. Core data structures
   - Implement `Node` with `left`/`right` pointers; a priority queue for building the tree with deterministic comparison.

7. Code generation
   - Walk the tree recursively to assign bit sequences; in the single-symbol case, assign a placeholder code (`0`). Ensure canonical traversal so codes are stable.

8. Header insertion and update
   - Write a header with a placeholder padding byte, write payload, then update the header padding (or write padding before payload if padding can be computed from final bitCount). We chose to calculate total bits during Pass 1 to write the final padding byte immediately in the header, avoiding the need for `Seek` on the output stream.

9. Decoding bitstream
   - Read header, reconstruct tree, then read payload bytes. For each byte, iterate bits MSB-first: `(b >> (7-i)) & 1`. Walk the tree per-bit; upon reaching a leaf, write that symbol and reset to root. Use the `expected_count` from metadata to know exactly when to stop, ignoring trailing padding bits.

10. Error detection
    - Validate header magic, ensure declared count of symbols matches readable entries, and during decode, verify total decoded symbols equals the sum of frequencies from header; otherwise return an error.

11. Verification (The Final Step)
    - Implement unit tests to confirm functional correctness and measure throughput.
    - Validate against the 50MB performance target and ensure sub-5-second encoding.
    - Confirm the Go implementation is substantially faster and more memory-efficient than the Python baseline.

12. Optional: single-pass streaming
    - If single-pass is a strict requirement, explore adaptive Huffman (FGK or Vitter) or a separate frequency producer upstream that writes a compact frequency header before payload so encoder can read frequencies and stream payload without buffering the full source.

## External References

- Huffman coding (Wikipedia): https://en.wikipedia.org/wiki/Huffman_coding
- Go `container/heap` usage: https://pkg.go.dev/container/heap
- Go binary encoding (`encoding/binary`): https://pkg.go.dev/encoding/binary
- Bit manipulation techniques (efficient packing): https://en.wikipedia.org/wiki/Bit_field#Packing_and_unpacking
- Adaptive Huffman (FGK algorithm): https://en.wikipedia.org/wiki/Adaptive_Huffman_coding
- Canonical Huffman codes (deterministic serialization): https://en.wikipedia.org/wiki/Canonical_Huffman_code
