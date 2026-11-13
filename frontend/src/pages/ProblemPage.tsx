import { lazy, Suspense, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { useProblem, useSubmit } from "../api/hooks";
import { useAuth } from "../auth/AuthProvider";
import { ErrorMessage } from "../components/ErrorMessage";
import { DEFAULT_LANGUAGE, LANGUAGES, languageInfo, type LanguageId } from "../languages";

// Monaco is large: load it only on problem pages, as a separate chunk.
const CodeEditor = lazy(() => import("../components/CodeEditor").then((m) => ({ default: m.CodeEditor })));

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
    <article>
      {contest && (
        <p className="notice">
          Contest <Link to={`/contests/${contest}`}>{contest}</Link>
          {label && <> · problem {label}</>}: your submissions count for the leaderboard.
        </p>
      )}
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
        <h2>Your solution</h2>
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
        <Suspense fallback={<p role="status">Loading editor…</p>}>
          <CodeEditor
            value={source}
            onChange={setSource}
            label="Solution source code"
            language={languageInfo(language).monaco}
          />
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
