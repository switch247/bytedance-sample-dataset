# Test Cases — Before & After

This document lists the planned test cases to run against both the existing Python implementation (the "before" baseline) and the migrated Go implementation (the "after" target). Test names will be reused across implementations to verify parity and improvements.

 NOTE: this is only here to show how tests can be planned ahead of time to align with requirements.

- **test_roundtrip_random_10kb**: Compress and then decompress a random 10KB byte payload; assert the output equals the original.

- **test_roundtrip_all_bytes_once**: Input containing one instance of every possible byte (0x00..0xFF); compress+decompress round-trip equality.

- **test_empty_input**: Compress and decompress an empty input; verify behavior (empty output or well-formed header) and no crash.

- **test_single_byte_repeated**: Input containing a single repeated byte (e.g., 1MB of 0x41); verify correct round-trip and that header encodes single-symbol alphabet.

- **test_small_inputs**: Multiple sub-cases for inputs of length 1, 2, and 3 bytes; verify correct decoding and padding handling.

- **test_padding_metadata**: Verify the header’s padding metadata matches the encoded payload, and that decoder ignores padding bits to recover exact original length.

- **test_header_serialization_deterministic**: Repeated compression of the same input produces the same serialized header (determinism check).

- **test_truncated_stream**: Feed a truncated/cut-off compressed stream to the decoder and assert a deterministic error is raised (header or payload corruption).

- **test_corrupted_header**: Corrupt bits in the header and verify the decoder reports a header-corruption error.

- **test_memory_profile_small_buffer**: Compress a large sample (e.g., 50MB) using small read buffers and assert peak memory usage grows modestly (smoke check).

- **test_performance_50mb**: Measure encode/decode time for a 50MB sample to record baseline throughput (Python) and assert improvement for Go in the after-suite. Go implementation MUST be significantly faster.
- **test_bitwise_integrity**: (Go only) Indirectly Validate bit-packing efficiency. The compressed size should be close to the theoretical entropy limit for random/skewed inputs, and significantly smaller than a string-based "0101..." representation (which would be 8x larger).

- **test_streaming_io**: Use stream-based reader/writer with small buffers (e.g., 64B chunks) to ensure the implementation works without loading whole file into memory. The implementation MUST NOT read the entire stream into memory at once. For `repository_after`, this is a critical requirement.
- **test_large_file_streaming**: (Go only) Validate that a file larger than available RAM (simulated or actual) can be processed. Since actual RAM exhaustion is hard to test in CI, we will verify that memory usage remains constant (O(1)) relative to input size processing a large (e.g., 100MB+) generated stream.

- **test_codes_consistency**: Validate that generated code lengths are consistent with frequency expectations (higher-frequency bytes get shorter codes); sanity check only.

Notes:
- These tests are written to run against both Python and Go implementations; the Go "after" tests will include performance assertions and stricter memory/throughput targets.
- For cross-language compatibility tests, include cases that compress in one language and decompress in the other.

