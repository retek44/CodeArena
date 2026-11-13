import { expect, test } from "@playwright/test";
import { ProblemsPage } from "../pages/pages";
import { expectNoSeriousViolations } from "../support/a11y";

// Post-deploy checks against a real environment: CloudFront -> S3 (SPA) and
// CloudFront -> ALB -> ECS -> RDS (API). Anonymous and read-only, so they are safe
// to run against prod too.
//
//   E2E_BASE_URL=https://dxxxx.cloudfront.net npx playwright test

test("the seeded problem list renders and is accessible", async ({ page }) => {
  await new ProblemsPage(page).open();
  await expect(page.getByRole("link", { name: "A + B" })).toBeVisible();
  await expectNoSeriousViolations(page);
});

test("deep links are served by the SPA routing function", async ({ page }) => {
  const response = await page.goto("/problems/a-plus-b");
  expect(response?.status()).toBe(200);
  await expect(page.getByRole("heading", { level: 1, name: "A + B" })).toBeVisible();
});

test("security headers are set on the SPA and on the API", async ({ request }) => {
  for (const path of ["/", "/api/problems"]) {
    const response = await request.get(path);
    expect(response.status(), path).toBe(200);
    const headers = response.headers();
    expect(headers["content-security-policy"], path).toContain("script-src 'self'");
    expect(headers["strict-transport-security"], path).toContain("max-age=31536000");
    expect(headers["x-frame-options"], path).toBe("DENY");
    expect(headers["x-content-type-options"], path).toBe("nosniff");
  }
});

test("API responses are never served from the edge cache", async ({ request }) => {
  await request.get("/api/problems");
  const second = await request.get("/api/problems");
  expect(second.headers()["x-cache"] ?? "").not.toMatch(/^Hit/);
});

test("plain HTTP is redirected to HTTPS", async ({ request, baseURL }) => {
  const response = await request.get(`${(baseURL ?? "").replace(/^https:/, "http:")}/`, { maxRedirects: 0 });
  expect(response.status()).toBe(301);
  expect(response.headers()["location"]).toMatch(/^https:\/\//);
});

test("the load balancer cannot be used without CloudFront", async ({ request }) => {
  const alb = process.env.E2E_ALB_URL;
  test.skip(!alb, "E2E_ALB_URL is not set");
  // The security group admits CloudFront only, so from here the connection should
  // not even open; if it does, the listener must still refuse the missing secret header.
  const outcome = await request
    .get(`${alb}/api/problems`, { timeout: 5_000, maxRedirects: 0 })
    .then((response) => response.status(), () => "unreachable");
  expect([403, "unreachable"]).toContain(outcome);
});
