import { Link, useParams } from "react-router";
import type { Submission } from "../api/client";
import { useHint, useSubmission } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
import { SubmissionStatus, VerdictBadge } from "../components/Verdict";
import { languageLabel } from "../languages";

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
        Problem <Link to={`/problems/${s.problem}`}>{s.problem}</Link> · {languageLabel(s.language)} ·{" "}
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

      {hintAvailable(s) && <HintPanel submissionId={s.id} />}

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

// Verdicts a hint can help with (the API answers 409 for the others).
const HINT_VERDICTS: ReadonlySet<string> = new Set(["WA", "RE", "TLE", "MLE", "OLE"]);

function hintAvailable(s: Submission): boolean {
  return s.status === "done" && s.verdict != null && HINT_VERDICTS.has(s.verdict);
}

function HintPanel({ submissionId }: { submissionId: string }) {
  const hint = useHint(submissionId);

  return (
    <section aria-labelledby="hint-heading">
      <h2 id="hint-heading">Stuck?</h2>
      {hint.data ? (
        <blockquote className="hint" aria-live="polite">
          <p>{hint.data.hint}</p>
          <footer className="meta">AI hint · it points at the problem, it does not solve it.</footer>
        </blockquote>
      ) : (
        <>
          <p className="meta">Get a short hint about the likely mistake. You can ask for 3 hints per hour.</p>
          <button type="button" onClick={() => hint.mutate()} disabled={hint.isPending}>
            {hint.isPending ? "Thinking…" : "Get a hint"}
          </button>
        </>
      )}
      {hint.isError && <ErrorMessage error={hint.error} />}
    </section>
  );
}
