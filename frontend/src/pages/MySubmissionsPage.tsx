import { Link } from "react-router";
import { useMySubmissions } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
import { SubmissionStatus } from "../components/Verdict";

export function MySubmissionsPage() {
  const query = useMySubmissions();
  const items = query.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <section>
      <h1>My submissions</h1>
      {query.isPending && <p role="status">Loading submissions…</p>}
      {query.isError && <ErrorMessage error={query.error} />}
      {query.isSuccess && items.length === 0 && (
        <p>
          No submissions yet. <Link to="/">Pick a problem</Link>.
        </p>
      )}
      {items.length > 0 && (
        <table>
          <caption className="visually-hidden">My submissions, newest first</caption>
          <thead>
            <tr>
              <th scope="col">Submitted</th>
              <th scope="col">Problem</th>
              <th scope="col">Result</th>
            </tr>
          </thead>
          <tbody>
            {items.map((s) => (
              <tr key={s.id}>
                <td>
                  <Link to={`/submissions/${s.id}`}>{new Date(s.created_at).toLocaleString()}</Link>
                </td>
                <td>{s.problem}</td>
                <td>
                  <SubmissionStatus submission={s} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {query.hasNextPage && (
        <button type="button" onClick={() => void query.fetchNextPage()} disabled={query.isFetchingNextPage}>
          {query.isFetchingNextPage ? "Loading…" : "Load more"}
        </button>
      )}
    </section>
  );
}
