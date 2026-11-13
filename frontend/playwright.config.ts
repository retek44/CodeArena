import { defineConfig, devices } from "@playwright/test";

// Two targets:
//  - local (default): the full docker compose stack (API on :8080 with AUTH_MODE=dev);
//    Playwright builds the SPA and serves it with `vite preview`, whose proxy forwards
//    /api to the API. Runs every spec except e2e/deployed.
//  - deployed: E2E_BASE_URL points at an AWS environment (CloudFront URL); runs only
//    e2e/deployed, the anonymous post-deploy checks.
const allBrowsers = process.env.PW_ALL_BROWSERS === "1";
const deployedUrl = process.env.E2E_BASE_URL;

export default defineConfig({
  testDir: deployedUrl ? "e2e/deployed" : "e2e",
  testIgnore: deployedUrl ? [] : ["deployed/**"],
  timeout: 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  // One retry in CI: a test that passes on retry is reported as flaky, not hidden.
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }], ["junit", { outputFile: "test-results/junit.xml" }]] : "list",
  use: {
    baseURL: deployedUrl ?? "http://localhost:4173",
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
  webServer: deployedUrl
    ? undefined
    : {
        command: "npm run build && npm run preview",
        url: "http://localhost:4173",
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        env: { VITE_AUTH_MODE: "dev" },
      },
});
