import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { json, mockFetch, problem, renderApp } from "../test/render";
import { aPlusB, submission } from "../test/fixtures";

describe("ProblemPage", () => {
  it("shows the statement, limits and samples", async () => {
    const api = mockFetch({ "GET /api/problems/a-plus-b": () => json(aPlusB) });
    renderApp("/problems/a-plus-b", { fetch: api.fetch });

    expect(await screen.findByRole("heading", { level: 1, name: "A + B" })).toBeInTheDocument();
    expect(screen.getByText(/time limit 1000 ms/)).toBeInTheDocument();
    expect(screen.getByText("Read two integers and print their sum.")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Input 1" })).toBeInTheDocument();
  });

  it("asks anonymous users to sign in instead of showing Submit", async () => {
    const api = mockFetch({ "GET /api/problems/a-plus-b": () => json(aPlusB) });
    renderApp("/problems/a-plus-b", { fetch: api.fetch, user: null });

    expect(await screen.findByRole("link", { name: "Sign in" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Submit" })).not.toBeInTheDocument();
  });

  it("submits the editor content with an Idempotency-Key and opens the submission", async () => {
    const created = submission();
    const api = mockFetch({
      "GET /api/problems/a-plus-b": () => json(aPlusB),
      "POST /api/submissions": () => json(created, 202),
      [`GET /api/submissions/${created.id}`]: () => json(created),
    });
    const { router } = renderApp("/problems/a-plus-b", { fetch: api.fetch });

    const editor = await screen.findByLabelText("Solution source code");
    await userEvent.clear(editor);
    await userEvent.type(editor, "print(3)");
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));

    expect(await screen.findByRole("heading", { name: "Submission" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(`/submissions/${created.id}`);

    const post = api.requests.find((r) => r.method === "POST");
    expect(post?.headers.get("Idempotency-Key")).toMatch(/^[0-9a-f-]{36}$/);
    expect(post?.headers.get("X-Dev-User")).toBe("alice");
    expect(await post?.json()).toEqual({ problem: "a-plus-b", language: "python", source: "print(3)" });
  });

  it("explains the rate limit with the Retry-After time", async () => {
    const api = mockFetch({
      "GET /api/problems/a-plus-b": () => json(aPlusB),
      "POST /api/submissions": () => problem(429, "at most 10 submissions per 1m0s", { "Retry-After": "42" }),
    });
    renderApp("/problems/a-plus-b", { fetch: api.fetch });

    await userEvent.click(await screen.findByRole("button", { name: "Submit" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Try again in 42 seconds");
  });

  it("shows the API error for an unknown problem", async () => {
    const api = mockFetch({ "GET /api/problems/nope": () => problem(404, "problem not found") });
    renderApp("/problems/nope", { fetch: api.fetch });

    expect(await screen.findByRole("alert")).toHaveTextContent("problem not found");
  });
});
