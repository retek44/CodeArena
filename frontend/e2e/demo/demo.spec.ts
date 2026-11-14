import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";
import { LoginPage, ProblemPage, ProblemsPage } from "../pages/pages";

// The README demo (make demo-video): one user story, slowed down and captioned.
// Not part of the regression suite (playwright.config.ts ignores e2e/demo).

const golden = (file: string) =>
  readFileSync(new URL(`../../../tests/golden/a-plus-b/solutions/${file}`, import.meta.url), "utf8");

const pause = (page: Page, ms = 1500) => page.waitForTimeout(ms);

/** Shows a caption bar at the bottom of the page (survives client-side navigation). */
async function caption(page: Page, text: string) {
  await page.evaluate((t) => {
    let bar = document.getElementById("demo-caption");
    if (!bar) {
      bar = document.createElement("div");
      bar.id = "demo-caption";
      bar.setAttribute("aria-hidden", "true");
      Object.assign(bar.style, {
        position: "fixed",
        left: "50%",
        bottom: "24px",
        transform: "translateX(-50%)",
        maxWidth: "80%",
        padding: "10px 18px",
        borderRadius: "8px",
        background: "rgba(17, 24, 39, 0.9)",
        color: "#fff",
        font: "600 18px system-ui, sans-serif",
        zIndex: "2147483647",
        pointerEvents: "none",
      });
      document.body.appendChild(bar);
    }
    bar.textContent = t;
  }, text);
  await pause(page, 1800);
}

/** Types the solution into Monaco so the viewer sees it written; falls back to setValue. */
async function typeSolution(page: Page, problem: ProblemPage, source: string) {
  await problem.setSource("");
  await page.locator(".monaco-editor .view-lines").click();
  for (const [i, line] of source.trimEnd().split("\n").entries()) {
    if (i > 0) {
      await page.keyboard.press("Enter");
      await page.keyboard.press("Home");
    }
    await page.keyboard.type(line, { delay: 35 });
  }
  const typed = await page.evaluate(
    () => (window as unknown as Record<string, { getValue(): string }>).__codeArenaEditor?.getValue(),
  );
  // Auto-closing brackets or auto-indent can change typed text; the demo must submit the real file.
  if (typed?.trimEnd() !== source.trimEnd()) {
    await problem.setSource(source);
  }
}

test("CodeArena demo", async ({ page }) => {
  const user = `ada-${Date.now().toString(36).slice(-4)}`;

  await page.goto("/login");
  await caption(page, "CodeArena: an online judge on AWS (local stack, development sign-in)");
  await new LoginPage(page).signInAs(user);

  const problems = new ProblemsPage(page);
  await problems.open();
  await caption(page, "Problems by difficulty");
  await problems.openProblem("A + B");
  const problem = new ProblemPage(page);
  await expect(page.getByRole("heading", { level: 1, name: "A + B" })).toBeVisible();
  await caption(page, "Statement and examples");

  await caption(page, "A first try: wrong on negative numbers");
  await typeSolution(page, problem, golden("wa.py"));
  let submission = await problem.submit();
  await caption(page, "Queued, judged by a worker in a sandboxed runner, pushed back live");
  await submission.expectVerdict("WA", "Wrong answer");
  await caption(page, "Verdict for every test case");

  await caption(page, "Stuck? Ask for a hint (Bedrock on AWS, a fake model locally)");
  await submission.askForHint();
  await pause(page, 2500);

  await problem.open("a-plus-b");
  await caption(page, "Fix it and submit again");
  await typeSolution(page, problem, golden("ac.py"));
  submission = await problem.submit();
  await submission.expectVerdict("AC", "Accepted");
  await caption(page, "Accepted on all test cases");

  await page.goto("/contests");
  await caption(page, "Contests: phases from the clock, ICPC scoring, live leaderboard");
  await page.getByRole("link", { name: "Weekly Contest 1" }).click();
  await page.getByRole("button", { name: "Register" }).click();
  await expect(page.getByText("You are registered.")).toBeVisible();
  await caption(page, "Registered; contest submissions count for the leaderboard");
  await page.getByRole("link", { name: "A + B" }).click();
  await problem.setSource(golden("ac.py"));
  submission = await problem.submit();
  await submission.expectVerdict("AC", "Accepted");

  await page.goto("/contests/weekly-1");
  const row = page.getByRole("table", { name: /Leaderboard/ }).getByRole("row").filter({ hasText: user });
  await expect(row).toContainText(user, { timeout: 15_000 });
  await row.scrollIntoViewIfNeeded();
  await caption(page, `${user} is on the leaderboard: 1 solved`);
  await pause(page, 2000);

  // The video is complete only after the context closes.
  await page.context().close();
  await page.video()?.saveAs("test-results/demo.webm");
});
