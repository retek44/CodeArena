import { defineConfig, devices } from "@playwright/test";

// Browser tests against the full local stack: `docker compose up -d --build --wait`
// (API on :8080 with AUTH_MODE=dev); Playwright builds the SPA and serves it with
// `vite preview`, whose proxy forwards /api to the API.
const allBrowsers = process.env.PW_ALL_BROWSERS === "1";

export default defineConfig({
  testDir: "e2e",
  timeout: 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  // One retry in CI: a test that passes on retry is reported as flaky, not hidden.
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }], ["junit", { outputFile: "test-results/junit.xml" }]] : "list",
  use: {
    baseURL: "http://localhost:4173",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    ...(allBrowsers
      ? [
          { name: "firefox", use: { ...devices["Desktop Firefox"] } },
          { name: "webkit", use: { ...devices["Desktop Safari"] } },
        ]
      : []),
  ],
  webServer: {
    command: "npm run build && npm run preview",
    url: "http://localhost:4173",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: { VITE_AUTH_MODE: "dev" },
  },
});
