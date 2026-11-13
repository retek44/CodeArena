import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";

// WCAG 2.1 A/AA rules. Serious and critical findings fail the test; the rest are reported.
export async function expectNoSeriousViolations(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    // Monaco renders its own editor DOM; its accessibility is Microsoft's, not ours.
    .exclude(".monaco-editor")
    .analyze();
  const serious = results.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  expect(serious.map((v) => `${v.id}: ${v.help} (${v.nodes.length} nodes)`)).toEqual([]);
}
