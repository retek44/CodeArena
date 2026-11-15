import { lazy, Suspense, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { useProblem, useSubmit } from "../api/hooks";
import { useAuth } from "../auth/AuthProvider";
import { ErrorMessage } from "../components/ErrorMessage";
import { Flap } from "../components/Flap";
import { DEFAULT_LANGUAGE, LANGUAGES, languageInfo, type LanguageId } from "../languages";

// Monaco is large: load it only on problem pages, as a separate chunk.
const CodeEditor = lazy(() => import("../components/CodeEditor").then((m) => ({ default: m.CodeEditor })));
// The Markdown renderer is a separate chunk too: the problem list does not need it.
const Statement = lazy(() => import("../components/Statement").then((m) => ({ default: m.Statement })));

export function ProblemPage() {
  const { slug = "" } = useParams();
  // Opened from a contest page: submissions count for that contest.
  const [search] = useSearchParams();
  const contest = search.get("contest") ?? undefined;
  const label = search.get("label");
  const problem = useProblem(slug);
  const submit = useSubmit();
  const auth = useAuth();
  const navigate = useNavigate();
  const [language, setLanguage] = useState<LanguageId>(DEFAULT_LANGUAGE);
  // One draft per language: switching back and forth keeps what was typed.
  const [drafts, setDrafts] = useState<Record<LanguageId, string>>(
    () => Object.fromEntries(LANGUAGES.map((l) => [l.id, l.starter])) as Record<LanguageId, string>,
  );
  const source = drafts[language];
  const setSource = (value: string) => setDrafts((d) => ({ ...d, [language]: value }));

  if (problem.isPending) return <p role="status">Loading problem…</p>;
  if (problem.isError) return <ErrorMessage error={problem.error} />;
  const p = problem.data;

  const onSubmit = () => {
    submit.mutate(
      { problem: p.slug, language, source, contest, idempotencyKey: crypto.randomUUID() },
      { onSuccess: (s) => void navigate(`/submissions/${s.id}`) },
    );
  };

  return (
    <article className="problem-page">
      <div className="problem-text">
        {contest && (
          <p className="notice">
            Contest <Link to={`/contests/${contest}`}>{contest}</Link>
            {label && <> · problem {label}</>}: your submissions count for the leaderboard.
          </p>
        )}
        <h1>{p.title}</h1>
        <ul className="limits" aria-label="Limits">
          <li>
            <Flap>{p.difficulty}</Flap>
          </li>
          <li>
            <Flap>time limit {p.time_limit_ms} ms</Flap>
          </li>
          <li>
            <Flap>memory limit {p.memory_limit_mb} MB</Flap>
          </li>
        </ul>
        <Suspense fallback={<p role="status">Loading statement…</p>}>
          <Statement markdown={p.statement_md} />
        </Suspense>

        {p.samples.length > 0 && (
          <section className="samples">
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
      </div>

      <section className="problem-work" aria-labelledby="solution-heading">
        <div className="work-bar">
          <h2 id="solution-heading">Your solution</h2>
          <label className="language-picker">
            Language{" "}
            <select value={language} onChange={(e) => setLanguage(e.target.value as LanguageId)}>
              {LANGUAGES.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.label}
                </option>
              ))}
            </select>
          </label>
          {auth.user ? (
            <button type="button" className="primary" onClick={onSubmit} disabled={submit.isPending || !source.trim()}>
              {submit.isPending ? "Submitting…" : "Submit"}
            </button>
          ) : (
            <p className="work-hint">
              <Link to="/login">Sign in</Link> to submit a solution.
            </p>
          )}
        </div>
        {submit.isError && <ErrorMessage error={submit.error} />}
        <Suspense fallback={<p role="status" className="editor-loading">Loading editor…</p>}>
          <CodeEditor
            value={source}
            onChange={setSource}
            label="Solution source code"
            language={languageInfo(language).monaco}
          />
        </Suspense>
      </section>
    </article>
  );
}
