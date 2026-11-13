import { useState } from "react";
import { Link } from "react-router";
import type { Difficulty } from "../api/client";
import { useProblems } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";

export function ProblemsPage() {
  const [difficulty, setDifficulty] = useState<Difficulty | "">("");
  const problems = useProblems(difficulty || undefined);
  const items = problems.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <section>
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

      {problems.isPending && <p role="status">Loading problems…</p>}
      {problems.isError && <ErrorMessage error={problems.error} />}
      {problems.isSuccess && items.length === 0 && <p>No problems yet.</p>}

      {items.length > 0 && (
        <table>
          <caption className="visually-hidden">Problems</caption>
          <thead>
            <tr>
              <th scope="col">Title</th>
              <th scope="col">Difficulty</th>
              <th scope="col">Tags</th>
            </tr>
          </thead>
          <tbody>
            {items.map((p) => (
              <tr key={p.slug}>
                <td>
                  <Link to={`/problems/${p.slug}`}>{p.title}</Link>
                </td>
                <td>{p.difficulty}</td>
                <td>{p.tags?.join(", ")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {problems.hasNextPage && (
        <button type="button" onClick={() => void problems.fetchNextPage()} disabled={problems.isFetchingNextPage}>
          {problems.isFetchingNextPage ? "Loading…" : "Load more"}
        </button>
      )}
    </section>
  );
}
