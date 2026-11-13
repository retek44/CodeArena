import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { json, mockFetch, renderApp } from "../test/render";

const summary = (slug: string, difficulty = "easy") => ({ slug, title: slug.toUpperCase(), difficulty, tags: ["math"] });

describe("ProblemsPage", () => {
  it("loads further pages with the cursor", async () => {
    const api = mockFetch({
      "GET /api/problems": (req) =>
        new URL(req.url).searchParams.get("cursor") === "c1"
          ? json({ items: [summary("p-3")], next_cursor: null })
          : json({ items: [summary("p-1"), summary("p-2")], next_cursor: "c1" }),
    });
    renderApp("/", { fetch: api.fetch });

    expect(await screen.findByRole("link", { name: "P-1" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));

    expect(await screen.findByRole("link", { name: "P-3" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  it("filters by difficulty through the API", async () => {
    const api = mockFetch({
      "GET /api/problems": (req) => {
        const d = new URL(req.url).searchParams.get("difficulty");
        return json({ items: d === "hard" ? [summary("hard-one", "hard")] : [summary("easy-one")], next_cursor: null });
      },
    });
    renderApp("/", { fetch: api.fetch });

    await screen.findByRole("link", { name: "EASY-ONE" });
    await userEvent.selectOptions(screen.getByLabelText("Difficulty"), "hard");

    expect(await screen.findByRole("link", { name: "HARD-ONE" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "EASY-ONE" })).not.toBeInTheDocument();
  });
});
