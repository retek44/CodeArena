import { defineConfig, devices } from "@playwright/test";
import base from "./playwright.config";

// Records the README demo (make demo-video) against the local compose stack, like the
// local E2E suite: one narrated run in Chromium with video on (pauses are in the spec).
export default defineConfig({
  ...base,
  testDir: "e2e/demo",
  testIgnore: [],
  timeout: 300_000,
  retries: 0,
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  outputDir: "test-results/demo-run",
  use: {
    ...base.use,
    viewport: { width: 1280, height: 720 },
    video: { mode: "on", size: { width: 1280, height: 720 } },
    trace: "off",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 720 } } }],
});
