import { useState } from "react";
import { Link } from "react-router";
import type { Difficulty } from "../api/client";
import { useProblems } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
import { Flap } from "../components/Flap";

export function ProblemsPage() {
  const [difficulty, setDifficulty] = useState<Difficulty | "">("");
  const problems = useProblems(difficulty || undefined);
  const items = problems.data?.pages.flatMap((p) => p.items) ?? [];
  // An empty column is noise: the Tags column shows only when some problem has tags.
  const withTags = items.some((p) => (p.tags?.length ?? 0) > 0);

  return (
    <section>
      <div className="page-head">
        <h1>Problems</h1>
        <label className="filter">
          Difficulty{" "}
          <select value={difficulty} onChange={(e) => setDifficulty(e.target.value as Difficulty | "")}>
            <option value="">All</option>
            <option value="easy">Easy</option>
            <option value="medium">Medium</option>
            <option value="hard">Hard</option>
          </select>
        </label>
      </div>

      {problems.isPending && <p role="status">Loading problems…</p>}
      {problems.isError && <ErrorMessage error={problems.error} />}
      {problems.isSuccess && items.length === 0 && <p>No problems yet.</p>}

      {items.length > 0 && (
        <table className="board-list">
          <caption className="visually-hidden">Problems</caption>
          <thead>
            <tr>
              <th scope="col">Title</th>
              <th scope="col">Difficulty</th>
              {withTags && <th scope="col">Tags</th>}
            </tr>
          </thead>
          <tbody>
            {items.map((p) => (
              <tr key={p.slug}>
                <td className="title">
                  <Link to={`/problems/${p.slug}`}>{p.title}</Link>
                </td>
                <td>
                  <Flap>{p.difficulty}</Flap>
                </td>
                {withTags && <td className="tags">{p.tags?.length ? p.tags.join(", ") : "—"}</td>}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {problems.hasNextPage && (
        <button type="button" className="load-more" onClick={() => void problems.fetchNextPage()} disabled={problems.isFetchingNextPage}>
          {problems.isFetchingNextPage ? "Loading…" : "Load more"}
        </button>
      )}
    </section>
  );
}
