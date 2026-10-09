const test = require("node:test");
const assert = require("node:assert/strict");
const { resolve } = require("../dist/config");

test("requires an api key and a service name", () => {
  assert.throws(() => resolve({}, {}), /apiKey is required/);
  assert.throws(() => resolve({ apiKey: "k" }, {}), /serviceName is required/);
});

test("reads configuration from the environment and applies defaults", () => {
  const c = resolve({}, { RESOLVEX_API_KEY: "k", RESOLVEX_SERVICE: "svc", RESOLVEX_ENDPOINT: "https://ingest.example.com//", NODE_ENV: "production" });
  assert.equal(c.endpoint, "https://ingest.example.com");
  assert.equal(c.environment, "production");
  assert.equal(c.autoInstrument, true);
  assert.equal(c.captureConsole, false);
});

test("explicit options beat the environment", () => {
  const c = resolve({ apiKey: "a", serviceName: "s", environment: "staging", captureConsole: true }, { RESOLVEX_API_KEY: "b", RESOLVEX_ENVIRONMENT: "prod" });
  assert.equal(c.apiKey, "a");
  assert.equal(c.environment, "staging");
  assert.equal(c.captureConsole, true);
});
