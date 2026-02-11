# Migration Documentation: Express Gateway → Fastify Gateway

## Overview

This document records the migration of a monolithic Express.js REST gateway to a production-hardened Fastify implementation. It captures the goal, requirements, what is working, improvement goals, the migration plan, and the verification tests used to prove endpoint parity and stronger validation.

## Understanding

- **Starting State**: `repository_before/server.js` contains a single-file Express gateway with three endpoints: `/telemetry`, `/user/:id`, and `/transform`.
- **Goal**: Refactor to Fastify for lower overhead and higher throughput while preserving behavior.
- **Requirements summarized**:
  - Keep 100% endpoint parity.
  - Use Fastify JSON Schema validation (AJV) for `POST /telemetry` and `POST /transform`.
  - Define response schemas for all endpoints (fast-json-stringify).
  - Enforce `deviceId` as string and `sensorValue` as number with automatic 400 responses.
  - Use async handlers and disable unnecessary logging for low-spec hardware.
  - Add a clean JSON validation error handler.
  - Provide unit tests and a performance verification command (autocannon).

## Questions to Understand

- Is a 2x/3x RPS improvement required in CI, or is an offline benchmark acceptable?
- Should the validation error response shape be strict (schema) or only machine-readable?
- Are we allowed to adjust AJV strict mode settings to remove warnings?
- Do we need a formal performance test in CI, or is a documented command sufficient?

## What Is Working (Current State)

- `repository_after/server.js` implements Fastify with schema validation, response schemas, and a custom validation error handler.
- AJV type coercion is disabled so invalid `deviceId` types are rejected as required.
- Tests run in-process via Jest + Supertest/Fastify `inject`, avoiding flaky port usage.
- Evaluation runner (`evaluation/evaluation.js`) executes before/after tests, prints emoji results, and writes a dated report under `evaluation/YYYY-MM-DD/HH-MM-SS/report.json`.

## What We Need to Improve (Goals)

- Optional: suppress AJV strict-mode warnings if noisy in CI.
- Optional: automate actual autocannon runs in evaluation when desired.
- Optional: run containerized benchmarks under CPU/memory limits (Compose or Docker run) for fair comparisons.

## Migration Plan (multi-step, with explanations)

1. **Define schema-first behavior**
   - Why: Fastify performance benefits depend on precompiled validation and serialization.
   - Actions: Define request/response JSON Schemas for `/telemetry`, `/user/:id`, `/transform`.

2. **Implement Fastify endpoints with async handlers**
   - Why: Keep non-blocking behavior and preserve parity.
   - Actions: Mirror request/response shapes and mapping logic in async handlers.

3. **Add custom validation error handling**
   - Why: Provide clean, machine-readable errors for clients.
   - Actions: Use `setErrorHandler` and return a JSON payload on validation errors.

4. **Introduce strict validation behavior**
   - Why: Enforce `deviceId` as string without coercion.
   - Actions: Configure AJV to disable coercion and enable `allErrors`.

5. **Add tests for parity and improvements**
   - Why: Prove behavior is preserved and improvements are real.
   - Actions: Use Jest with Supertest/Fastify `inject` to test before/after behavior in-process.

6. **Document performance verification**
   - Why: Provide a repeatable benchmark command.
   - Actions: Include an autocannon command with rate and worker limits and a threshold check.

## Migration Execution Details

During the implementation phase, several technical challenges were addressed to ensure stability and accuracy:

1.  **Standardized Test Isolation**:
    - Challenge: Custom CLI flags like `--before` caused Jest to fail with "Unrecognized CLI Parameter".
    - Fix: Updated `package.json` to use standard name filtering (`jest -t before` / `jest -t after`) and updated `unified.test.js` to detect targets via string matching on `process.argv`.

2.  **Windows Reliability for Evaluation**:
    - Challenge: `spawnSync` on Windows struggled with absolute paths and file-locking delays when reading JSON results.
    - Fix: Rewrote `evaluation.js` to use a temporary directory with relative paths and implemented an aggressive JSON extraction fallback that parses results directly from terminal output if the file is locked or missing.

3.  **Benchmarking Stability**:
    - Challenge: `MaxListenersExceededWarning` and excessive `autocannon` logging caused buffer issues and EPIPE errors.
    - Fix: Increased `EventEmitter.defaultMaxListeners` to `0` (unlimited) and muted `console.error/warn` during high-load benchmarks to prevent log-bloat.

4.  **Automatic Workspace Cleanup**:
    - Actions: Integrated an `fs.rmSync` routine in the evaluation script to purge the `evaluation/tmp` directory after every run, ensuring the project remains clean.

5.  **Strict Type Enforcement**:
    - Configuration: Set `coerceTypes: false` in Fastify's AJV options to guarantee that `deviceId` is strictly a string, fulfilling the requirement for stronger validation over the legacy Express implementation.

## Verification Testcases — design and rationale

Test selection principles:
- Preserve functional parity.
- Ensure validation and stricter type handling in Fastify.
- Provide a documented performance check.

Key testcases implemented:

- **POST /telemetry success**
  - Valid payload returns `201` and `{ status, id }`.

- **POST /telemetry invalid type**
  - Invalid `sensorValue` returns `400`.

- **POST /telemetry non-string deviceId**
  - Before: allowed (legacy behavior). After: rejected (schema validation).

- **GET /user/:id**
  - Returns metadata with expected fields and types.

- **POST /transform mapping**
  - Applies uppercase UID, multiplier, and boolean coercion.

- **POST /transform invalid items**
  - Returns `400` for non-array `items`.

- **Performance command check**
  - Autocannon command includes rate and worker limits and a threshold value.

## Implementation Notes

- Fastify runs with `logger: false` for reduced overhead.
- AJV options: `coerceTypes: false`, `allErrors: true`.
- Tests are in-process and avoid external ports for stability.
- Evaluation prints emoji-marked outcomes and stores a dated JSON report.

## External References

- Fastify documentation: https://www.fastify.io/docs/latest/
- Fastify schema validation: https://www.fastify.io/docs/latest/Reference/Validation-and-Serialization/
- AJV JSON Schema: https://ajv.js.org/
- Autocannon: https://github.com/mcollina/autocannon
