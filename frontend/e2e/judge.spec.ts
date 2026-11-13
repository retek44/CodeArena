import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { LoginPage, ProblemPage, ProblemsPage } from "./pages/pages";

const golden = (file: string) =>
  readFileSync(new URL(`../../tests/golden/a-plus-b/solutions/${file}`, import.meta.url), "utf8");

// A fresh user per test keeps tests independent (submission rate limit, history).
const uniqueUser = () => `pw-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;

test.beforeEach(async ({ page }) => {
  await new LoginPage(page).signInAs(uniqueUser());
});

test("a correct solution is accepted on every test case", async ({ page }) => {
  const problems = new ProblemsPage(page);
  await problems.open();
  await problems.openProblem("A + B");

  const problem = new ProblemPage(page);
  await problem.setSource(golden("ac.py"));
  const submission = await problem.submit();

  await submission.expectVerdict("AC", "Accepted");
  await expect(submission.cases.getByRole("row")).toHaveCount(5); // header + 4 cases
  await expect(submission.cases.getByText("Wrong answer")).toHaveCount(0);
});

test("a wrong answer shows which test cases failed", async ({ page }) => {
  const problem = new ProblemPage(page);
  await problem.open("a-plus-b");
  await problem.setSource(golden("wa.py"));
  const submission = await problem.submit();

  await submission.expectVerdict("WA", "Wrong answer");
  await expect(submission.cases.getByText("Accepted").first()).toBeVisible();
});

test("a syntax error shows the compiler output", async ({ page }) => {
  const problem = new ProblemPage(page);
  await problem.open("a-plus-b");
  await problem.setSource(golden("ce.py"));
  const submission = await problem.submit();

  await submission.expectVerdict("CE", "Compile error");
  await expect(page.getByRole("heading", { name: "Compiler output" })).toBeVisible();
  await expect(page.getByText("SyntaxError")).toBeVisible();
});

test("the submission appears in My submissions", async ({ page }) => {
  const problem = new ProblemPage(page);
  await problem.open("a-plus-b");
  await problem.setSource(golden("ac.py"));
  const submission = await problem.submit();
  await submission.expectVerdict("AC", "Accepted");

  await page.getByRole("link", { name: "My submissions" }).click();
  const rows = page.getByRole("table", { name: "My submissions, newest first" }).getByRole("row");
  await expect(rows).toHaveCount(2);
  await expect(rows.nth(1)).toContainText("AC Accepted");
});
