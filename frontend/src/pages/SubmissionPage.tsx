import { useState } from "react";
import { Link, useParams } from "react-router";
import { useSubmission } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
import { Flap } from "../components/Flap";
import { SubmissionStatus, VerdictBadge } from "../components/Verdict";
import { languageLabel } from "../languages";

/** Case numbers on the board are two digits wide: 01, 02, … */
const caseNumber = (n: number) => String(n).padStart(2, "0");

export function SubmissionPage() {
  const { id = "" } = useParams();
  const query = useSubmission(id);
  // Results already there when the page opened stand still; later ones flip in as they arrive.
  const [casesAtLoad, setCasesAtLoad] = useState<number | null>(null);
  if (query.data && casesAtLoad === null) setCasesAtLoad(query.data.cases.length);

  if (query.isPending) return <p role="status">Loading submission…</p>;
  if (query.isError) return <ErrorMessage error={query.error} />;
  const s = query.data;
  const loaded = casesAtLoad ?? s.cases.length;
  const judging = !s.verdict;
  // The board stands while the submission waits; a compile error never gets one.
  const showBoard = s.cases.length > 0 || judging;

  return (
    <article className="submission-page">
      <h1>Submission</h1>
      <p>
        Problem <Link to={`/problems/${s.problem}`}>{s.problem}</Link> · {languageLabel(s.language)} ·{" "}
        {new Date(s.created_at).toLocaleString()}
      </p>
      <p className="result" aria-live="polite">
        Result: <SubmissionStatus submission={s} />{" "}
        {s.runtime_ms != null && <Flap className="meta">{s.runtime_ms} ms</Flap>}{" "}
        {s.memory_kb != null && <Flap className="meta">{Math.round(s.memory_kb / 1024)} MB</Flap>}
      </p>

      {s.compile_output && (
        <section className="compile-output">
          <h2>Compiler output</h2>
          <pre>{s.compile_output}</pre>
        </section>
      )}

      {s.cases.length === 0 && judging && (
        <p className="meta">Test case results appear here as they are judged.</p>
      )}

      {showBoard && (
        // Until the first result arrives the board only pictures what is coming.
        <table className="cases" aria-hidden={s.cases.length === 0 ? true : undefined}>
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
            {s.cases.map((c, i) => (
              <tr key={c.index}>
                <td className="case">{caseNumber(c.index)}</td>
                <td className="verdict">
                  <VerdictBadge verdict={c.verdict} flipOnMount={i >= loaded} />
                </td>
                <td className="num">{c.runtime_ms ?? "–"}</td>
                <td className="num">{c.memory_kb ?? "–"}</td>
              </tr>
            ))}
            {judging && (
              // The next case on the board: it waits, then flips to its verdict.
              <tr className="next" aria-hidden="true">
                <td className="case">{caseNumber(s.cases.length + 1)}</td>
                <td className="verdict">
                  <Flap tone={s.status === "running" ? "run" : "wait"} state={s.status} className="badge">
                    {s.status === "running" ? "Running" : "Queued"}
                  </Flap>
                </td>
                <td className="num">—</td>
                <td className="num">—</td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </article>
  );
}
