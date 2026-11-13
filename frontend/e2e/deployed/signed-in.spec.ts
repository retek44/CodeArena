import { readFileSync } from "node:fs";
import { randomUUID } from "node:crypto";
import { expect, test, type APIRequestContext } from "@playwright/test";

// Signed-in post-deploy checks: a real Cognito access token, a real judge run through
// CloudFront -> ALB -> API -> outbox -> SQS -> worker (Fargate) -> runner Lambda.
// The pipeline creates a temporary Cognito user, signs it in with
// ADMIN_USER_PASSWORD_AUTH, passes the access token in E2E_ACCESS_TOKEN and deletes
// the user afterwards. Without the token the tests are skipped.

const token = process.env.E2E_ACCESS_TOKEN;
test.skip(!token, "E2E_ACCESS_TOKEN not set (the deploy pipeline signs in a test user)");

// Cold starts: worker scale-out from zero (queue alarm, Fargate start) and the first
// runner invocation take a few minutes on a fresh environment.
test.describe.configure({ mode: "serial", timeout: 6 * 60_000 });

const golden = (problem: string, file: string) =>
  readFileSync(new URL(`../../../tests/golden/${problem}/solutions/${file}`, import.meta.url), "utf8");

const auth = () => ({ Authorization: `Bearer ${token}` });

interface Submission {
  id: string;
  status: string;
  verdict: string | null;
  language: string;
}

async function submitAndWait(request: APIRequestContext, problem: string, language: string, source: string) {
  const created = await request.post("/api/submissions", {
    headers: { ...auth(), "Idempotency-Key": randomUUID() },
    data: { problem, language, source },
  });
  expect(created.status(), await created.text()).toBe(202);
  const { id } = (await created.json()) as Submission;

  const read = async () => {
    const response = await request.get(`/api/submissions/${id}`, { headers: auth() });
    expect(response.status()).toBe(200);
    return (await response.json()) as Submission;
  };
  await expect
    .poll(async () => (await read()).status, { timeout: 5 * 60_000, intervals: [2_000, 5_000, 10_000] })
    .toMatch(/^(done|failed)$/);
  return read();
}

test("the API rejects requests without a token", async ({ request }) => {
  const response = await request.post("/api/submissions", {
    headers: { "Idempotency-Key": randomUUID() },
    data: { problem: "a-plus-b", language: "python", source: "print(1)" },
  });
  expect(response.status()).toBe(401);
});

test("a Python solution is judged AC by the runner Lambda", async ({ request }) => {
  const result = await submitAndWait(request, "a-plus-b", "python", golden("a-plus-b", "ac.py"));
  expect(result).toMatchObject({ status: "done", verdict: "AC", language: "python" });
});

test("a C++ solution is compiled and judged by the C++ runner", async ({ request }) => {
  const result = await submitAndWait(request, "a-plus-b", "cpp", golden("a-plus-b", "wa_int_overflow.cpp"));
  expect(result).toMatchObject({ status: "done", verdict: "WA", language: "cpp" });
});

test("a custom checker accepts a different correct answer", async ({ request }) => {
  const result = await submitAndWait(request, "pair-sum", "python", golden("pair-sum", "ac_from_the_end.py"));
  expect(result).toMatchObject({ status: "done", verdict: "AC" });
});

test("the user's own submissions are listed", async ({ request }) => {
  const response = await request.get("/api/users/me/submissions", { headers: auth() });
  expect(response.status()).toBe(200);
  const page = (await response.json()) as { items: Submission[] };
  expect(page.items.length).toBeGreaterThanOrEqual(1);
});
