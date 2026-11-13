import { defineConfig } from "vitest/config";

// Pact consumer tests (src/api/pact): the real request functions against the Pact mock
// server. They write pacts/codearena-spa-codearena-api.json, which the Go provider test
// verifies (backend/internal/httpapi/pact_provider_test.go).
export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.pact.test.ts"],
    globalSetup: ["./src/api/pact/global-setup.ts"],
    fileParallelism: false,
    testTimeout: 30_000,
  },
});
