import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { json, mockFetch, renderApp } from "../test/render";

const contest = (over: Record<string, unknown> = {}) => ({
  slug: "weekly-1",
  title: "Weekly Contest 1",
  starts_at: "2026-11-13T10:00:00Z",
  freeze_at: "2026-11-13T11:40:00Z",
  ends_at: "2026-11-13T12:00:00Z",
  phase: "running",
  description_md: "Three warm-up problems.",
  problems: [
    { label: "A", slug: "a-plus-b", title: "A + B" },
    { label: "B", slug: "pair-sum", title: "Pair sum" },
  ],
  participants: 2,
  registered: false,
  ...over,
});

const board = (over: Record<string, unknown> = {}) => ({
  contest: "weekly-1",
  phase: "running",
  frozen: false,
  labels: ["A", "B"],
  rows: [
    {
      rank: 1,
      username: "bob",
      solved: 2,
      penalty: 55,
      problems: [
        { label: "A", solved_minute: 10, rejected: 1, pending: 0 },
        { label: "B", solved_minute: 25, rejected: 0, pending: 0 },
      ],
    },
    {
      rank: 2,
      username: "alice",
      solved: 0,
      penalty: 0,
      problems: [
        { label: "A", solved_minute: null, rejected: 2, pending: 0 },
        { label: "B", solved_minute: null, rejected: 0, pending: 1 },
      ],
    },
  ],
  ...over,
});

describe("ContestPage", () => {
  it("registers and then links the problems into the contest", async () => {
    let registered = false;
    const api = mockFetch({
      "GET /api/contests/weekly-1": () => json(contest({ registered })),
      "POST /api/contests/weekly-1/register": () => {
        registered = true;
        return new Response(null, { status: 204 });
      },
      "GET /api/contests/weekly-1/leaderboard": () => json(board()),
    });
    renderApp("/contests/weekly-1", { fetch: api.fetch });

    expect(await screen.findByRole("heading", { name: "Weekly Contest 1" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "A + B" })).toHaveAttribute("href", "/problems/a-plus-b");

    await userEvent.click(screen.getByRole("button", { name: "Register" }));
    expect(await screen.findByText("You are registered.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "A + B" })).toHaveAttribute(
      "href",
      "/problems/a-plus-b?contest=weekly-1&label=A",
    );
  });

  it("shows the leaderboard in ICPC notation", async () => {
    const api = mockFetch({
      "GET /api/contests/weekly-1": () => json(contest()),
      "GET /api/contests/weekly-1/leaderboard": () => json(board()),
    });
    renderApp("/contests/weekly-1", { fetch: api.fetch });

    const table = await screen.findByRole("table", { name: /Leaderboard/ });
    const rows = within(table).getAllByRole("row");
    expect(rows).toHaveLength(3);
    expect(rows[1]).toHaveTextContent("1bob255+1 10'+ 25'");
    expect(rows[2]).toHaveTextContent("2alice00-2?");
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
  });

  it("says when the leaderboard is frozen and hides problems before the start", async () => {
    const api = mockFetch({
      "GET /api/contests/weekly-1": () => json(contest({ phase: "scheduled", problems: [] })),
      "GET /api/contests/weekly-1/leaderboard": () => json(board({ frozen: true, phase: "frozen" })),
    });
    renderApp("/contests/weekly-1", { fetch: api.fetch });

    expect(await screen.findByRole("note")).toHaveTextContent("frozen");
    expect(screen.getByText("The problems appear when the contest starts.")).toBeInTheDocument();
  });

  it("submits from a contest problem with the contest", async () => {
    const api = mockFetch({
      "GET /api/problems/a-plus-b": () =>
        json({
          slug: "a-plus-b",
          title: "A + B",
          difficulty: "easy",
          statement_md: "Add.",
          time_limit_ms: 1000,
          memory_limit_mb: 64,
          tags: [],
          samples: [],
        }),
      "POST /api/submissions": async (req) => {
        const body = (await req.json()) as Record<string, unknown>;
        return json({ id: "6f1c1d64-0d8c-4f39-9c55-2a0d0b8f9e11", ...body, status: "queued", verdict: null }, 202);
      },
    });
    renderApp("/problems/a-plus-b?contest=weekly-1&label=A", { fetch: api.fetch });

    expect(await screen.findByText(/your submissions count for the leaderboard/)).toBeInTheDocument();
    await userEvent.click(await screen.findByRole("button", { name: /Submit/ }));
    await waitFor(() => expect(api.requests.some((r) => r.method === "POST")).toBe(true));
    const post = api.requests.find((r) => r.method === "POST");
    expect(await post?.json()).toMatchObject({ problem: "a-plus-b", contest: "weekly-1" });
  });
});
