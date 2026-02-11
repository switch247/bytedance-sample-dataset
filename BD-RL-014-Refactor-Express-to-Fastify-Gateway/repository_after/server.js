"use strict";

// Fastify gateway with schema-first validation and serialization.
const buildServer = () => {
  const fastify = require("fastify")({
    logger: false,
    ajv: {
      customOptions: {
        coerceTypes: false,
        allErrors: true,
      },
    },
  });

const telemetrySchema = {
  body: {
    type: "object",
    required: ["deviceId", "sensorValue"],
    additionalProperties: false,
    properties: {
      deviceId: { type: "string" },
      sensorValue: { type: "number" },
      timestamp: { type: ["number", "string"], nullable: true },
    },
  },
  response: {
    201: {
      type: "object",
      required: ["status", "id"],
      additionalProperties: false,
      properties: {
        status: { type: "string" },
        id: { type: "string" },
      },
    },
  },
};

const userSchema = {
  params: {
    type: "object",
    required: ["id"],
    additionalProperties: false,
    properties: {
      id: { type: "string" },
    },
  },
  response: {
    200: {
      type: "object",
      required: ["id", "role", "lastLogin", "preferences"],
      additionalProperties: false,
      properties: {
        id: { type: "string" },
        role: { type: "string" },
        lastLogin: { type: "number" },
        preferences: {
          type: "object",
          required: ["theme", "notifications"],
          additionalProperties: false,
          properties: {
            theme: { type: "string" },
            notifications: { type: "boolean" },
          },
        },
      },
    },
  },
};

const transformSchema = {
  body: {
    type: "object",
    required: ["items"],
    additionalProperties: false,
    properties: {
      items: {
        type: "array",
        items: {
          type: "object",
          required: ["id", "value"],
          additionalProperties: true,
          properties: {
            id: { type: ["string", "number"] },
            value: { type: "number" },
            status: {},
          },
        },
      },
    },
  },
  response: {
    200: {
      type: "object",
      required: ["processed", "count"],
      additionalProperties: false,
      properties: {
        processed: {
          type: "array",
          items: {
            type: "object",
            required: ["uid", "val", "active"],
            additionalProperties: false,
            properties: {
              uid: { type: "string" },
              val: { type: "number" },
              active: { type: "boolean" },
            },
          },
        },
        count: { type: "number" },
      },
    },
  },
};

  fastify.setErrorHandler(async (error, _request, reply) => {
  if (error.validation) {
    return reply.status(400).send({
      error: "Bad Request",
      message: "Validation failed",
      details: error.validation,
    });
  }
  reply.send(error);
});

// 1. High-speed telemetry ingestion
  fastify.post("/telemetry", { schema: telemetrySchema }, async (request, reply) => {
  const { deviceId } = request.body;
  return reply.code(201).send({ status: "recorded", id: deviceId });
});

// 2. Parameterized user metadata retriever
  fastify.get("/user/:id", { schema: userSchema }, async (request) => {
  const userId = request.params.id;
  return {
    id: userId,
    role: "admin",
    lastLogin: Date.now(),
    preferences: { theme: "dark", notifications: true },
  };
});

// 3. JSON-heavy data transformer
  fastify.post("/transform", { schema: transformSchema }, async (request) => {
  const { items } = request.body;
  const transformed = items.map((item) => ({
    uid: item.id.toString().toUpperCase(),
    val: item.value * 1.15,
    active: !!item.status,
  }));
  return { processed: transformed, count: transformed.length };
});

  return fastify;
};

module.exports = { buildServer };

if (require.main === module) {
  const server = buildServer();
  const PORT = process.env.PORT ? Number(process.env.PORT) : 3000;
  server.listen({ port: PORT, host: "0.0.0.0" }, (err, address) => {
    if (err) {
      server.log.error(err);
      process.exit(1);
    }
    // eslint-disable-next-line no-console
    console.log(`Fastify gateway running on ${address}`);
  });
}
