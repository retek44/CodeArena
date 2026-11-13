import { expect, type Locator, type Page } from "@playwright/test";

// Page objects: tests describe what a user does; selectors live here.
// Locators use roles and accessible names, so they double as accessibility checks.

export class LoginPage {
  constructor(private readonly page: Page) {}

  async signInAs(name: string) {
    await this.page.goto("/login");
    await this.page.getByLabel("User name").fill(name);
    await this.page.getByRole("button", { name: "Sign in" }).click();
    await expect(this.page.getByRole("button", { name: "Sign out" })).toBeVisible();
  }
}

export class ProblemsPage {
  constructor(private readonly page: Page) {}

  async open() {
    await this.page.goto("/");
    await expect(this.page.getByRole("heading", { level: 1, name: "Problems" })).toBeVisible();
  }

  async openProblem(title: string) {
    await this.page.getByRole("link", { name: title }).click();
  }
}

export class ProblemPage {
  readonly submitButton: Locator;

  constructor(private readonly page: Page) {
    this.submitButton = page.getByRole("button", { name: "Submit" });
  }

  async open(slug: string) {
    await this.page.goto(`/problems/${slug}`);
    await expect(this.page.getByRole("heading", { level: 1 })).toBeVisible();
  }

  async chooseLanguage(label: string) {
    await this.page.getByLabel("Language").selectOption({ label });
  }

  /** Replaces the editor content. insertText bypasses Monaco's auto-indent and auto-close. */
  async setSource(source: string) {
    const lines = this.page.locator(".monaco-editor .view-lines");
    await lines.click();
    await this.page.keyboard.press("ControlOrMeta+A");
    await this.page.keyboard.press("Delete");
    await this.page.keyboard.insertText(source);
  }

  async submit(): Promise<SubmissionPage> {
    await this.submitButton.click();
    await expect(this.page).toHaveURL(/\/submissions\/[0-9a-f-]{36}$/);
    return new SubmissionPage(this.page);
  }
}

export class SubmissionPage {
  readonly result: Locator;
  readonly cases: Locator;
  readonly hintButton: Locator;
  readonly hint: Locator;

  constructor(page: Page) {
    this.result = page.getByText(/^Result:/);
    this.cases = page.getByRole("table", { name: "Test cases" });
    this.hintButton = page.getByRole("button", { name: "Get a hint" });
    this.hint = page.getByRole("region", { name: "Stuck?" }).getByRole("blockquote");
  }

  /** Asks for the AI hint and returns its text. */
  async askForHint(): Promise<string> {
    await this.hintButton.click();
    await expect(this.hint).toBeVisible({ timeout: 30_000 });
    return (await this.hint.innerText()).trim();
  }

  /** Waits until the polling page shows a final verdict. */
  async expectVerdict(code: string, label: string) {
    await expect(this.result).toContainText(`${code} ${label}`, { timeout: 45_000 });
  }
}
