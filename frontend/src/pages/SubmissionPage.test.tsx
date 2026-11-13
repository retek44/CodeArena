import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { json, mockFetch, problem, renderApp } from "../test/render";
import { submission } from "../test/fixtures";

describe("SubmissionPage", () => {
  it("polls until the verdict arrives, then stops", async () => {
    const id = submission().id;
    const responses = [
      submission({ status: "queued" }),
      submission({
        status: "done",
        verdict: "WA",
        runtime_ms: 12,
        memory_kb: 9216,
        judged_at: "2026-11-13T10:00:02Z",
        cases: [
          { index: 1, verdict: "AC", runtime_ms: 10, memory_kb: 9000 },
          { index: 2, verdict: "WA", runtime_ms: 12, memory_kb: 9216 },
        ],
      }),
    ];
    let calls = 0;
    const api = mockFetch({
      [`GET /api/submissions/${id}`]: () => json(responses[Math.min(calls++, responses.length - 1)]),
    });
    renderApp(`/submissions/${id}`, { fetch: api.fetch });

    expect(await screen.findByText("In queue…")).toBeInTheDocument();
    const table = await screen.findByRole("table", { name: "Test cases" }, { timeout: 3000 });
    expect(screen.getByText(/^Result:/)).toHaveTextContent("WA Wrong answer");
    expect(within(table).getAllByRole("row")).toHaveLength(3);
    expect(screen.getByText(/12 ms/)).toBeInTheDocument();

    const callsAtVerdict = calls;
    await new Promise((r) => setTimeout(r, 1500));
    expect(calls).toBe(callsAtVerdict);
  });

  it("shows compiler output for a compile error", async () => {
    const s = submission({ status: "done", verdict: "CE", compile_output: "SyntaxError: '(' was never closed" });
    const api = mockFetch({ [`GET /api/submissions/${s.id}`]: () => json(s) });
    renderApp(`/submissions/${s.id}`, { fetch: api.fetch });

    expect(await screen.findByText("Compile error")).toBeInTheDocument();
    expect(screen.getByText(/was never closed/)).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("asks for a hint after a wrong answer and shows it", async () => {
    const s = submission({ status: "done", verdict: "WA", cases: [{ index: 1, verdict: "WA" }] });
    let hintCalls = 0;
    const api = mockFetch({
      [`GET /api/submissions/${s.id}`]: () => json(s),
      [`POST /api/submissions/${s.id}/hint`]: () => {
        hintCalls++;
        return json({ submission_id: s.id, hint: "Check the sum of two large numbers.", status: "ok", created_at: "2026-11-13T10:00:03Z" });
      },
    });
    renderApp(`/submissions/${s.id}`, { fetch: api.fetch });

    await userEvent.click(await screen.findByRole("button", { name: "Get a hint" }));
    expect(await screen.findByText("Check the sum of two large numbers.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Get a hint" })).not.toBeInTheDocument();
    expect(hintCalls).toBe(1);
  });

  it("shows the hint limit", async () => {
    const s = submission({ status: "done", verdict: "TLE" });
    const api = mockFetch({
      [`GET /api/submissions/${s.id}`]: () => json(s),
      [`POST /api/submissions/${s.id}/hint`]: () => problem(429, "at most 3 hints per 1h0m0s", { "Retry-After": "600" }),
    });
    renderApp(`/submissions/${s.id}`, { fetch: api.fetch });

    await userEvent.click(await screen.findByRole("button", { name: "Get a hint" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Too many requests. Try again in 10 minutes.");
  });

  it.each(["AC", "CE"] as const)("offers no hint for %s", async (verdict) => {
    const s = submission({ status: "done", verdict });
    const api = mockFetch({ [`GET /api/submissions/${s.id}`]: () => json(s) });
    renderApp(`/submissions/${s.id}`, { fetch: api.fetch });

    expect(await screen.findByText(/^Result:/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Get a hint" })).not.toBeInTheDocument();
  });
});
