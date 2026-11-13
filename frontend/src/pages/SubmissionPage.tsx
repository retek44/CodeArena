import { Link, useParams } from "react-router";
import { useSubmission } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
import { SubmissionStatus, VerdictBadge } from "../components/Verdict";

export function SubmissionPage() {
  const { id = "" } = useParams();
  const query = useSubmission(id);

  if (query.isPending) return <p role="status">Loading submission…</p>;
  if (query.isError) return <ErrorMessage error={query.error} />;
  const s = query.data;

  return (
    <article>
      <h1>Submission</h1>
      <p>
        Problem <Link to={`/problems/${s.problem}`}>{s.problem}</Link> · {s.language} ·{" "}
        {new Date(s.created_at).toLocaleString()}
      </p>
      <p className="result" aria-live="polite">
        Result: <SubmissionStatus submission={s} />
        {s.runtime_ms != null && <span className="meta"> · {s.runtime_ms} ms</span>}
        {s.memory_kb != null && <span className="meta"> · {Math.round(s.memory_kb / 1024)} MB</span>}
      </p>

      {s.compile_output && (
        <section>
          <h2>Compiler output</h2>
          <pre>{s.compile_output}</pre>
        </section>
      )}

      {s.cases.length > 0 && (
        <table>
          <caption>Test cases</caption>
          <thead>
            <tr>
              <th scope="col">#</th>
              <th scope="col">Verdict</th>
              <th scope="col">Time (ms)</th>
              <th scope="col">Memory (KB)</th>
            </tr>
          </thead>
          <tbody>
            {s.cases.map((c) => (
              <tr key={c.index}>
                <td>{c.index}</td>
                <td>
                  <VerdictBadge verdict={c.verdict} />
                </td>
                <td>{c.runtime_ms ?? "–"}</td>
                <td>{c.memory_kb ?? "–"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </article>
  );
}
