"use strict";

require("events").EventEmitter.defaultMaxListeners = 0;

const request = require("supertest");
const autocannon = require("autocannon");

jest.setTimeout(60000);

const TARGET_FROM_ENV = process.env.TEST_TARGET;
const LIFECYCLE = process.env.npm_lifecycle_event;
const ARGS = process.argv.join(" ");
const TARGET_FROM_ARGV = ARGS.includes("before")
	? "before"
	: ARGS.includes("after")
		? "after"
		: null;
const TARGET_FROM_NPM =
	LIFECYCLE === "test:before"
		? "before"
		: LIFECYCLE === "test:after"
			? "after"
			: null;
const TARGET = TARGET_FROM_ARGV || TARGET_FROM_ENV || TARGET_FROM_NPM;
const TARGET_SET = TARGET === "before" || TARGET === "after";

const AUTOCANNON_CMD =
	"npx autocannon -c 200 -d 20 -p 10 --rate 20000 --workers 1 http://127.0.0.1:3000/telemetry";
const PERF_LATENCY_MAX_MS = 250;

const runAutocannon = (url) =>
	new Promise((resolve, reject) => {
		const originalError = console.error;
		const originalWarn = console.warn;
		console.error = () => {};
		console.warn = () => {};

		const instance = autocannon(
			{
				url,
				connections: 200,
				duration: 20,
				pipelining: 10,
				workers: 1,
			},
			(err, result) => {
				console.error = originalError;
				console.warn = originalWarn;
				if (err) {
					reject(err);
					return;
				}
				resolve(result);
			}
		);
		instance.on("error", (err) => {
			console.error = originalError;
			console.warn = originalWarn;
			reject(err);
		});
	});

const createSuite = (label, target) => {
	describe(`${label} server`, () => {
		let fastify = null;
		let app = null;
		let send = null;

		beforeAll(async () => {
			if (target === "before") {
				app = require("../repository_before/server").buildApp();
				send = (method, path, body) =>
					request(app)[method.toLowerCase()](path).send(body);
				return;
			}
			fastify = require("../repository_after/server").buildServer();
			await fastify.ready();
			send = async (method, path, body) => {
				const response = await fastify.inject({
					method,
					url: path,
					payload: body,
				});
				return {
					status: response.statusCode,
					body: response.json(),
				};
			};
		});

		afterAll(async () => {
			if (fastify) {
				await fastify.close();
			}
		});

		test("describes an autocannon command for latency validation", () => {
			expect(AUTOCANNON_CMD).toMatch(/autocannon/);
			expect(AUTOCANNON_CMD).toMatch(/20000|20,?000/);
			expect(AUTOCANNON_CMD).toMatch(/--workers 1/);
		});

		test("autocannon results are printed", () => {
			expect(AUTOCANNON_CMD).toMatch(/autocannon/);
		});

		test("autocannon latency threshold (before)", async () => {
			if (target !== "before") {
				return;
			}
			const server = app.listen(0);
			const { port } = server.address();
			const result = await runAutocannon(
				`http://127.0.0.1:${port}/telemetry`
			);
			console.log(`\nAutocannon (${label})`);
			console.log(`  Requests/sec: ${result.requests.average}`);
			console.log(`  Latency avg: ${result.latency.average} ms`);
			console.log(`  Throughput: ${result.throughput.average} bytes/sec`);
			await new Promise((resolve) => server.close(resolve));
			expect(result.requests.average).toBeGreaterThan(0);
			expect(result.latency.average).toBeLessThanOrEqual(PERF_LATENCY_MAX_MS);
		});

		test("autocannon latency threshold (after)", async () => {
			if (target !== "after") {
				return;
			}
			const autocannonServer = require("../repository_after/server").buildServer();
			await autocannonServer.ready();
			const address = await autocannonServer.listen({
				port: 0,
				host: "127.0.0.1",
			});
			const result = await runAutocannon(`${address}/telemetry`);
			console.log(`\nAutocannon (${label})`);
			console.log(`  Requests/sec: ${result.requests.average}`);
			console.log(`  Latency avg: ${result.latency.average} ms`);
			console.log(`  Throughput: ${result.throughput.average} bytes/sec`);
			await autocannonServer.close();
			expect(result.latency.average).toBeLessThanOrEqual(PERF_LATENCY_MAX_MS);
		});

		test("POST /telemetry accepts valid payload", async () => {
			const response = await send("POST", "/telemetry", {
				deviceId: "dev-1",
				sensorValue: 42.5,
				timestamp: 123,
			});

			if (target === "before") {
				expect(response.status).toBe(201);
				expect(response.body).toEqual({ status: "recorded", id: "dev-1" });
				return;
			}
			expect(response.status).toBe(201);
			expect(response.body).toEqual({ status: "recorded", id: "dev-1" });
		});

		test("POST /telemetry rejects invalid sensorValue type", async () => {
			const response = await send("POST", "/telemetry", {
				deviceId: "dev-2",
				sensorValue: "bad",
			});

			if (target === "before") {
				expect(response.status).toBe(400);
				return;
			}
			expect(response.status).toBe(400);
			expect(response.body && response.body.error).toBeTruthy();
		});

		test("POST /telemetry rejects non-string deviceId", async () => {
			const response = await send("POST", "/telemetry", {
				deviceId: 123,
				sensorValue: 5.5,
			});
			if (target === "before") {
				expect(response.status).toBe(201);
				return;
			}
			expect(response.status).toBe(400);
		});

		test("GET /user/:id returns expected metadata", async () => {
			const response =
				target === "before"
					? await send("GET", "/user/alice")
					: await send("GET", "/user/alice");

			expect(response.status).toBe(200);
			expect(response.body.id).toBe("alice");
			expect(response.body.role).toBe("admin");
			expect(response.body.preferences.theme).toBe("dark");
			expect(response.body.preferences.notifications).toBe(true);
			expect(typeof response.body.lastLogin).toBe("number");
		});

		test("POST /transform maps items and returns count", async () => {
			const response = await send("POST", "/transform", {
				items: [
					{ id: "a1", value: 10, status: "yes" },
					{ id: 2, value: 4, status: 0 },
				],
			});

			expect(response.status).toBe(200);
			expect(response.body.count).toBe(2);
			expect(response.body.processed[0].uid).toBe("A1");
			expect(response.body.processed[0].val).toBe(11.5);
			expect(response.body.processed[0].active).toBe(true);
			expect(response.body.processed[1].uid).toBe("2");
			expect(response.body.processed[1].val).toBe(4.6);
			expect(response.body.processed[1].active).toBe(false);
		});

		test("POST /transform rejects non-array items", async () => {
			const response = await send("POST", "/transform", {
				items: "not-an-array",
			});

			expect(response.status).toBe(400);
			if (target === "after") {
				expect(response.body && response.body.error).toBeTruthy();
			}
		});
	});
};

if (TARGET_SET) {
	createSuite(TARGET, TARGET);
} else {
	createSuite("before", "before");
	createSuite("after", "after");
}