import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { AdminProblem, ProblemInput } from "../api/client";
import { json, mockFetch, problem, renderApp } from "../test/render";

const me = (username: string, admin: boolean) => ({ id: `dev-${username}`, username, admin });

const doubleIt: AdminProblem = {
  slug: "double-it",
  title: "Double It",
  difficulty: "easy",
  statement_md: "# Double It\n\nPrint `2n`.",
  time_limit_ms: 1000,
  memory_limit_mb: 64,
  checker: "exact",
  checker_tolerance: null,
  checker_source: null,
  published: false,
  test_cases: [
    { input: "2\n", output: "4\n", sample: true },
    { input: "21\n", output: "42\n", sample: false },
  ],
  version: 1,
  updated_at: "2026-11-15T10:00:00Z",
};

describe("admin pages", () => {
  it("are only for admins", async () => {
    const api = mockFetch({ "GET /api/users/me": () => json(me("alice", false)) });
    renderApp("/admin/problems", { fetch: api.fetch });

    expect(await screen.findByText("Only admins can manage problems.")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument();
    expect(api.requests.map((r) => new URL(r.url).pathname)).not.toContain("/api/admin/problems");
  });

  it("ask signed-out users to sign in", async () => {
    const api = mockFetch({});
    renderApp("/admin/problems/new", { fetch: api.fetch, user: null });

    expect(await screen.findByText(/as an admin to manage problems/)).toBeInTheDocument();
    expect(api.requests).toHaveLength(0);
  });

  it("list every problem with its version", async () => {
    const api = mockFetch({
      "GET /api/users/me": () => json(me("admin", true)),
      "GET /api/admin/problems": () =>
        json({
          items: [{ slug: "double-it", title: "Double It", difficulty: "easy", published: false, version: 3, updated_at: doubleIt.updated_at }],
        }),
    });
    renderApp("/admin/problems", { fetch: api.fetch, user: "admin" });

    const row = (await screen.findByRole("link", { name: "Double It" })).closest("tr");
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getAllByRole("cell").map((c) => c.textContent)).toEqual(
      expect.arrayContaining(["double-it", "No", "3"]),
    );
    expect(screen.getByRole("link", { name: "Admin" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "New problem" })).toHaveAttribute("href", "/admin/problems/new");
  });

  it("create a problem with test cases from files and open it", async () => {
    let sent: ProblemInput | undefined;
    const api = mockFetch({
      "GET /api/users/me": () => json(me("admin", true)),
      "POST /api/admin/problems": async (req) => {
        sent = (await req.json()) as ProblemInput;
        return json({ ...sent, version: 1, updated_at: doubleIt.updated_at }, 201);
      },
    });
    renderApp("/admin/problems/new", { fetch: api.fetch, user: "admin" });

    await userEvent.type(await screen.findByLabelText("Slug"), "double-it");
    await userEvent.type(screen.getByLabelText("Title"), "Double It");
    await userEvent.type(screen.getByLabelText("Markdown"), "Print **twice** n.");
    expect(await screen.findByText("twice", { selector: "strong" }, { timeout: 5000 })).toBeInTheDocument();

    await userEvent.upload(screen.getByLabelText(/Upload test case files/), [
      new File(["2\n"], "1.in"),
      new File(["4\n"], "1.out"),
      new File(["5\n"], "2.in"),
      new File(["10\n"], "2.out"),
      new File(["?"], "3.in"),
    ]);
    expect(await screen.findByText("Added 2 test cases. No .in/.out partner for: 3.in.")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Create problem" }));

    expect(await screen.findByText(/Problem created\. Version 1\./)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Edit problem" })).toBeInTheDocument();
    expect(screen.getByLabelText("Slug")).toBeDisabled();
    expect(sent).toMatchObject({
      slug: "double-it",
      title: "Double It",
      checker: "exact",
      checker_tolerance: null,
      checker_source: null,
      published: false,
      test_cases: [
        { input: "2\n", output: "4\n", sample: true },
        { input: "5\n", output: "10\n", sample: false },
      ],
    });
  });

  it("save a new version and keep only the settings of the chosen checker", async () => {
    let sent: ProblemInput | undefined;
    const api = mockFetch({
      "GET /api/users/me": () => json(me("admin", true)),
      "GET /api/admin/problems/double-it": () => json(doubleIt),
      "PUT /api/admin/problems/double-it": async (req) => {
        sent = (await req.json()) as ProblemInput;
        return json({ ...sent, version: 2, updated_at: doubleIt.updated_at });
      },
    });
    renderApp("/admin/problems/double-it", { fetch: api.fetch, user: "admin" });

    expect(await screen.findByText(/^Version 1\./)).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Checker"), "float");
    await userEvent.type(screen.getByLabelText("Tolerance"), "0.001");
    await userEvent.click(screen.getByRole("button", { name: "Remove test case 2" }));
    await userEvent.click(screen.getByRole("button", { name: "Add test case" }));
    const third = screen.getByRole("group", { name: "Test case 2" });
    await userEvent.type(within(third).getByLabelText("Input"), "-3");
    await userEvent.type(within(third).getByLabelText("Expected output"), "-6");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText(/Saved\. Version 2\./)).toBeInTheDocument();
    expect(sent).toMatchObject({
      checker: "float",
      checker_tolerance: 0.001,
      checker_source: null,
      test_cases: [
        { input: "2\n", output: "4\n", sample: true },
        { input: "-3", output: "-6", sample: false },
      ],
    });
  });

  it("explain a conflicting save", async () => {
    const api = mockFetch({
      "GET /api/users/me": () => json(me("admin", true)),
      "GET /api/admin/problems/double-it": () => json(doubleIt),
      "PUT /api/admin/problems/double-it": () => problem(409, "another save changed the problem"),
    });
    renderApp("/admin/problems/double-it", { fetch: api.fetch, user: "admin" });

    await userEvent.click(await screen.findByRole("button", { name: "Save" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("another save changed the problem");
    expect(screen.getByText(/Reload the page to see the latest version/)).toBeInTheDocument();
  });
});
