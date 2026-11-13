import { lazy, Suspense, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { useProblem, useSubmit } from "../api/hooks";
import { useAuth } from "../auth/AuthProvider";
import { ErrorMessage } from "../components/ErrorMessage";

// Monaco is large: load it only on problem pages, as a separate chunk.
const CodeEditor = lazy(() => import("../components/CodeEditor").then((m) => ({ default: m.CodeEditor })));

export const STARTER_CODE = "# Read from standard input, write to standard output.\n";

export function ProblemPage() {
  const { slug = "" } = useParams();
  const problem = useProblem(slug);
  const submit = useSubmit();
  const auth = useAuth();
  const navigate = useNavigate();
  const [source, setSource] = useState(STARTER_CODE);

  if (problem.isPending) return <p role="status">Loading problem…</p>;
  if (problem.isError) return <ErrorMessage error={problem.error} />;
  const p = problem.data;

  const onSubmit = () => {
    submit.mutate(
      { problem: p.slug, source, idempotencyKey: crypto.randomUUID() },
      { onSuccess: (s) => void navigate(`/submissions/${s.id}`) },
    );
  };

  return (
    <article>
      <h1>{p.title}</h1>
      <p className="meta">
        {p.difficulty} · time limit {p.time_limit_ms} ms · memory limit {p.memory_limit_mb} MB
      </p>
      <div className="statement">{p.statement_md}</div>

      {p.samples.length > 0 && (
        <section>
          <h2>Examples</h2>
          {p.samples.map((s, i) => (
            <div className="sample" key={i}>
              <div>
                <h3>Input {i + 1}</h3>
                <pre>{s.input}</pre>
              </div>
              <div>
                <h3>Output {i + 1}</h3>
                <pre>{s.output}</pre>
              </div>
            </div>
          ))}
        </section>
      )}

      <section>
        <h2>Your solution (Python)</h2>
        <Suspense fallback={<p role="status">Loading editor…</p>}>
          <CodeEditor value={source} onChange={setSource} label="Solution source code" />
        </Suspense>
        {submit.isError && <ErrorMessage error={submit.error} />}
        {auth.user ? (
          <button type="button" className="primary" onClick={onSubmit} disabled={submit.isPending || !source.trim()}>
            {submit.isPending ? "Submitting…" : "Submit"}
          </button>
        ) : (
          <p>
            <Link to="/login">Sign in</Link> to submit a solution.
          </p>
        )}
      </section>
    </article>
  );
}
