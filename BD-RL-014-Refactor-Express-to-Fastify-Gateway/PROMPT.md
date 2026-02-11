# Refactor Express to Fastify Gateway

## Problem Statement
The objective is to refactor a monolithic Express.js REST gateway into a production-hardened Fastify implementation. The legacy service currently suffers from high framework overhead, leading to excessive memory consumption and latency spikes on low-spec hardware. The new implementation must leverage Fastify's native optimizations, including AJV-based schema validation and the fast-json-stringify engine, to maximize throughput and achieve a significant increase in requests per second (RPS) while maintaining strict endpoint parity.

## Task Description
You are a Senior Infrastructure Engineer tasked with eliminating framework overhead to maximize performance on low-spec hardware. The business currently operates a legacy Express.js server that handles three core functions: high-speed telemetry ingestion, a parameterized user metadata retriever, and a JSON-heavy data transformer. 

The primary goal of this migration is to refactor the code to Fastify to achieve a 3x increase in Requests Per Second (RPS) while significantly lowering memory usage per request. To succeed, the new implementation must not only translate the routes but must leverage Fastify internal optimizations, specifically its native JSON Schema validation via AJV and the fast-json-stringify engine for response serialization. 

This is a one-file migration where you will take the provided monolithic Express server and output a production-hardened Fastify equivalent that maintains 100 percent endpoint parity while radically reducing execution latency. You must ensure that the schema definitions for inputs and outputs are explicitly defined to allow Fastify to pre-compile the serialization and validation logic. The resulting service should be capable of handling high-concurrency loads with minimal garbage collection pressure.

### Legacy Implementation (Express.js)

```javascript
const express = require('express');
const app = express();
app.use(express.json());

// 1. High-speed telemetry ingestion
app.post('/telemetry', (req, res) => {
  const { deviceId, sensorValue, timestamp } = req.body;
  if (!deviceId || typeof sensorValue !== 'number') {
    return res.status(400).json({ error: 'Invalid payload' });
  }
  // Logic: Log ingestion (Simulated)
  res.status(201).json({ status: 'recorded', id: deviceId });
});

// 2. Parameterized user metadata retriever
app.get('/user/:id', (req, res) => {
  const userId = req.params.id;
  // Logic: Mock data retrieval
  const userMetadata = {
    id: userId,
    role: 'admin',
    lastLogin: Date.now(),
    preferences: { theme: 'dark', notifications: true }
  };
  res.json(userMetadata);
});

// 3. JSON-heavy data transformer
app.post('/transform', (req, res) => {
  const { items } = req.body;
  if (!Array.isArray(items)) {
    return res.status(400).json({ error: 'Items must be an array' });
  }
  const transformed = items.map(item => ({
    uid: item.id.toString().toUpperCase(),
    val: item.value * 1.15,
    active: !!item.status
  }));
  res.json({ processed: transformed, count: transformed.length });
});

app.listen(3000, () => {
  console.log('Express gateway running on port 3000');
});
```

## Requirements

- **Refactor**: Implement all three legacy Express routes into a single-file Fastify server running on port 3000.
- **Validation**: Implement native JSON Schema validation (AJV) for the `POST /telemetry` and `POST /transform` endpoints.
- **Serialization**: Use Fastify response schemas (`fast-json-stringify`) for every endpoint to optimize JSON serialization.
- **Type Safety**: The `POST /telemetry` route must enforce that `sensorValue` is a number and `deviceId` is a string, returning a `400 Bad Request` via the AJV engine on failure.
- **Route Logic**: 
    - `GET /user/:id` must correctly extract the `id` parameter and return a serialized metadata object matching the legacy structure.
    - `POST /transform` must accept an array of items and perform mapping logic (UID uppercase, 1.15 value multiplier).
- **concurrency**: All routes must use asynchronous patterns to ensure the event loop remains unblocked.
- **Performance**:
    - Describe an `autocannon` command and verify the server achieves an average latency of 200 ms or less on a single core.
    - Configure optimized settings for low-spec hardware (e.g., disable unnecessary production logging).
- **Error Handling**: Implement a custom error handler to return clean, machine-readable JSON for validation errors.
- **Testing**: 
    - Provide a unit test suite (using `supertest` or Fastify's `.inject()`) to verify 100% logic parity.
    - Include a validation test proving that invalid data types to `/telemetry` result in a `400 Bad Request`.

## Technical Metadata

- **Difficulty**: Medium-Hard
- **Languages**: JavaScript, Node.js
- **Frameworks**: Express, Fastify
- **Libraries**: AJV, fast-json-stringify
- **Tools**: autocannon, npm
- **Core Concepts**: Framework Overhead Reduction, JSON Schema Validation, Optimized Serialization, Request Bottleneck Analysis, High-Throughput Middleware
- **Performance Metrics**:
    - Throughput: >5,000 RPS
    - P99 Latency: < 10ms
    - Memory Footprint: Minimal per request
- **Security Standards**: Schema-based Validation, Content-Type Enforcement