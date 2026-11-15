import { expect, test } from "@playwright/test";
import { AdminProblemPage, LoginPage, ProblemPage } from "./pages/pages";
import { expectNoSeriousViolations } from "./support/a11y";

// Dev users "admin-<name>" are admins (backend/internal/httpapi/auth.go).
const unique = () => `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;

test("a user who is not an admin cannot open the admin pages", async ({ page }) => {
  await new LoginPage(page).signInAs(`pw-${unique()}`);
  await page.goto("/admin/problems");

  await expect(page.getByText("Only admins can manage problems.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Admin" })).toHaveCount(0);
});

test("an admin creates a problem that judges submissions with its newest test cases", async ({ page }) => {
  const id = unique();
  const slug = `pw-double-${id}`;
  await new LoginPage(page).signInAs(`admin-pw-${id}`);
  await expect(page.getByRole("link", { name: "Admin" })).toBeVisible();

  const admin = new AdminProblemPage(page);
  await admin.openNew();
  await admin.fill({ slug, title: `Double ${id}`, statement: "Read `n`, print **2n**." });
  await expect(page.getByRole("region", { name: "Preview" }).locator("strong")).toHaveText("2n");
  await admin.uploadTestFiles({ "1.in": "2\n", "1.out": "4\n", "2.in": "21\n", "2.out": "42\n" });
  await expect(page.getByText("Added 2 test cases.")).toBeVisible();
  await page.getByLabel("Published (visible to everyone)").check();
  await expectNoSeriousViolations(page);
  await admin.save("Create problem");
  await admin.expectVersion(1);
  await expect(page).toHaveURL(new RegExp(`/admin/problems/${slug}$`));

  // A test case the doubling solution fails: version 2 judges it as a wrong answer.
  await page.getByRole("button", { name: "Add test case" }).click();
  const added = page.getByRole("group", { name: "Test case 3" });
  await added.getByLabel("Input").fill("5\n");
  await added.getByLabel("Expected output").fill("11\n");
  await admin.save("Save");
  await admin.expectVersion(2);

  const problem = new ProblemPage(page);
  await problem.open(slug);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(`Double ${id}`);
  await problem.setSource("print(2 * int(input()))\n");
  const submission = await problem.submit();
  await submission.expectVerdict("WA", "Wrong answer");
  await expect(submission.cases.getByRole("row")).toHaveCount(4); // header + 3 cases
});
