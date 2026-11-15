import { Link } from "react-router";
import { useAdminProblems } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
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
        <table>
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
                <td>
                  <Link to={`/admin/problems/${p.slug}`}>{p.title}</Link>
                </td>
                <td>
                  <code>{p.slug}</code>
                </td>
                <td>{p.difficulty}</td>
                <td>{p.published ? "Yes" : "No"}</td>
                <td>{p.version}</td>
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
