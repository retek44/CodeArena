import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { LoginPage, ProblemPage } from "./pages/pages";

// WCAG 2.1 A/AA rules. Serious and critical findings fail the test; the rest are reported.
async function expectNoSeriousViolations(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    // Monaco renders its own editor DOM; its accessibility is Microsoft's, not ours.
    .exclude(".monaco-editor")
    .analyze();
  const serious = results.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  expect(serious.map((v) => `${v.id}: ${v.help} (${v.nodes.length} nodes)`)).toEqual([]);
}

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
