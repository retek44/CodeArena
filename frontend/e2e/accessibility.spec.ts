import { expect, test } from "@playwright/test";
import { LoginPage, ProblemPage } from "./pages/pages";
import { expectNoSeriousViolations } from "./support/a11y";

test("problem list", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("link", { name: "A + B" })).toBeVisible();
  await expectNoSeriousViolations(page);
});

test("problem page with editor", async ({ page }) => {
  await new LoginPage(page).signInAs(`pw-a11y-${Date.now().toString(36)}`);
  await new ProblemPage(page).open("a-plus-b");
  await expectNoSeriousViolations(page);
});

test("sign-in page", async ({ page }) => {
  await page.goto("/login");
  await expectNoSeriousViolations(page);
});

test("contest page with leaderboard", async ({ page }) => {
  await page.goto("/contests/weekly-1");
  await expect(page.getByRole("heading", { name: "Leaderboard" })).toBeVisible();
  await expectNoSeriousViolations(page);
});
