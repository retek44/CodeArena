import { Link } from "react-router";
import { useAdminProblems } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
import { Flap } from "../components/Flap";
import { RequireAdmin } from "../components/RequireAdmin";

export function AdminProblemsPage() {
  return (
    <section>
      <h1>Manage problems</h1>
      <RequireAdmin>
        <ProblemList />
      </RequireAdmin>
    </section>
  );
}

function ProblemList() {
  const problems = useAdminProblems();

  return (
    <>
      <p>
        <Link to="/admin/problems/new">New problem</Link>
      </p>
      {problems.isPending && <p role="status">Loading problems…</p>}
      {problems.isError && <ErrorMessage error={problems.error} />}
      {problems.isSuccess && problems.data.length === 0 && <p>No problems yet.</p>}
      {problems.isSuccess && problems.data.length > 0 && (
        <table className="board-list">
          <caption className="visually-hidden">All problems</caption>
          <thead>
            <tr>
              <th scope="col">Title</th>
              <th scope="col">Slug</th>
              <th scope="col">Difficulty</th>
              <th scope="col">Published</th>
              <th scope="col">Version</th>
              <th scope="col">Updated</th>
            </tr>
          </thead>
          <tbody>
            {problems.data.map((p) => (
              <tr key={p.slug}>
                <td className="title">
                  <Link to={`/admin/problems/${p.slug}`}>{p.title}</Link>
                </td>
                <td>
                  <code>{p.slug}</code>
                </td>
                <td>
                  <Flap>{p.difficulty}</Flap>
                </td>
                <td>
                  <Flap tone={p.published ? "ok" : "wait"}>{p.published ? "Yes" : "No"}</Flap>
                </td>
                <td className="num">{p.version}</td>
                <td>
                  <time dateTime={p.updated_at}>{new Date(p.updated_at).toLocaleString()}</time>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}
