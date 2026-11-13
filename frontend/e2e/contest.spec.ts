import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { LoginPage, ProblemPage } from "./pages/pages";

const golden = (file: string) =>
  readFileSync(new URL(`../../tests/golden/a-plus-b/solutions/${file}`, import.meta.url), "utf8");

const uniqueUser = () => `pw-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;

// The seed creates "Weekly Contest 1" running from the first seed (tests/golden/contests.json).
test("register, solve a contest problem and appear on the leaderboard", async ({ page }) => {
  const user = uniqueUser();
  await new LoginPage(page).signInAs(user);

  await page.goto("/contests");
  await page.getByRole("link", { name: "Weekly Contest 1" }).click();
  await expect(page.getByRole("heading", { name: "Weekly Contest 1" })).toBeVisible();
  await page.getByRole("button", { name: "Register" }).click();
  await expect(page.getByText("You are registered.")).toBeVisible();

  await page.getByRole("link", { name: "A + B" }).click();
  await expect(page.getByText(/your submissions count for the leaderboard/)).toBeVisible();
  const problem = new ProblemPage(page);
  await problem.setSource(golden("ac.py"));
  const submission = await problem.submit();
  await submission.expectVerdict("AC", "Accepted");

  await page.goto("/contests/weekly-1");
  const row = page.getByRole("table", { name: /Leaderboard/ }).getByRole("row").filter({ hasText: user });
  await expect(row).toContainText(user, { timeout: 15_000 });
  await expect(row.getByRole("cell").nth(2)).toHaveText("1"); // solved
});
