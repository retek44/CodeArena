import { lazy, Suspense, useDeferredValue, useState, type ChangeEvent, type FormEvent } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router";
import { ApiError, type AdminProblem, type ProblemInput, type TestCase } from "../api/client";
import { useAdminProblem, useSaveProblem } from "../api/hooks";
import { ErrorMessage } from "../components/ErrorMessage";
import { RequireAdmin } from "../components/RequireAdmin";
import { pairTestFiles, testDataBytes } from "./testFiles";

// Loaded with the first preview, like on the problem page.
const Statement = lazy(() => import("../components/Statement").then((m) => ({ default: m.Statement })));

// Limits of the API (contracts/openapi.yaml, ProblemInput).
const MAX_TEST_CASES = 100;
const MAX_TEST_DATA_BYTES = 4 << 20;

type Checker = ProblemInput["checker"];

const EMPTY: ProblemInput = {
  slug: "",
  title: "",
  difficulty: "easy",
  statement_md: "",
  time_limit_ms: 1000,
  memory_limit_mb: 256,
  checker: "exact",
  checker_tolerance: null,
  checker_source: null,
  published: false,
  test_cases: [{ input: "", output: "", sample: true }],
};

export function AdminProblemPage() {
  const { slug } = useParams();
  return (
    <section>
      <h1>{slug ? "Edit problem" : "New problem"}</h1>
      <p>
        <Link to="/admin/problems">All problems</Link>
      </p>
      <RequireAdmin>{slug ? <EditProblem slug={slug} /> : <ProblemForm key="new" />}</RequireAdmin>
    </section>
  );
}

function EditProblem({ slug }: { slug: string }) {
  const problem = useAdminProblem(slug);
  if (problem.isPending) return <p role="status">Loading problem…</p>;
  if (problem.isError) return <ErrorMessage error={problem.error} />;
  return <ProblemForm key={slug} slug={slug} initial={problem.data} />;
}

function inputOf(p: AdminProblem): ProblemInput {
  return {
    slug: p.slug,
    title: p.title,
    difficulty: p.difficulty,
    statement_md: p.statement_md,
    time_limit_ms: p.time_limit_ms,
    memory_limit_mb: p.memory_limit_mb,
    checker: p.checker,
    checker_tolerance: p.checker_tolerance,
    checker_source: p.checker_source,
    published: p.published,
    test_cases: p.test_cases,
  };
}

function ProblemForm({ slug, initial }: { slug?: string; initial?: AdminProblem }) {
  const navigate = useNavigate();
  const location = useLocation();
  const save = useSaveProblem(slug);
  const [form, setForm] = useState<ProblemInput>(() => (initial ? inputOf(initial) : EMPTY));
  const [tolerance, setTolerance] = useState(initial?.checker_tolerance?.toString() ?? "");
  const [fileNotice, setFileNotice] = useState<{ text: string; error: boolean } | null>(null);
  const statement = useDeferredValue(form.statement_md);
  const current = save.data ?? initial;
  const version = current?.version;
  const created = (location.state as { created?: boolean } | null)?.created && !save.data;
  const bytes = testDataBytes(form.test_cases);

  const set = <K extends keyof ProblemInput>(key: K, value: ProblemInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }));
  const setCase = (i: number, change: Partial<TestCase>) =>
    set(
      "test_cases",
      form.test_cases.map((c, j) => (j === i ? { ...c, ...change } : c)),
    );

  const onFiles = async (e: ChangeEvent<HTMLInputElement>) => {
    const input = e.currentTarget;
    const files = await Promise.all([...(input.files ?? [])].map(async (f) => ({ name: f.name, text: await f.text() })));
    input.value = "";
    const { cases, unpaired } = pairTestFiles(files);
    // The one empty case of a new problem is a placeholder: replace it.
    const [first] = form.test_cases;
    const placeholder = form.test_cases.length === 1 && !first?.input && !first?.output;
    if (cases.length > 0) {
      set("test_cases", placeholder ? cases.map((c, i) => ({ ...c, sample: i === 0 })) : [...form.test_cases, ...cases]);
    }
    const added = `Added ${cases.length} test case${cases.length === 1 ? "" : "s"}.`;
    setFileNotice(
      unpaired.length > 0
        ? { text: `${added} No .in/.out partner for: ${unpaired.join(", ")}.`, error: true }
        : { text: added, error: false },
    );
  };

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    const value = Number(tolerance);
    const input: ProblemInput = {
      ...form,
      slug: form.slug.trim(),
      title: form.title.trim(),
      checker_tolerance: form.checker === "float" && tolerance.trim() && Number.isFinite(value) ? value : null,
      checker_source: form.checker === "custom" ? form.checker_source : null,
    };
    save.mutate(input, {
      onSuccess: (saved) => {
        if (!slug) void navigate(`/admin/problems/${saved.slug}`, { replace: true, state: { created: true } });
      },
    });
  };

  return (
    <form className="admin-form" onSubmit={onSubmit}>
      {version !== undefined && (
        <p className="notice" role="status">
          {created && "Problem created. "}
          {save.isSuccess && "Saved. "}
          Version {version}. A change of the limits, the checker or the test cases saves version {version + 1};
          earlier submissions keep the test cases they were judged with.
          {current?.published && (
            <>
              {" "}
              <Link to={`/problems/${current.slug}`}>Open the problem page</Link>
            </>
          )}
        </p>
      )}

      <div className="fields">
        <label>
          Slug
          <input
            value={form.slug}
            onChange={(e) => set("slug", e.target.value)}
            required
            pattern="[a-z0-9]+(-[a-z0-9]+)*"
            maxLength={64}
            disabled={slug !== undefined}
            aria-describedby="slug-help"
          />
        </label>
        <small id="slug-help">
          Lowercase letters, digits and dashes, e.g. <code>a-plus-b</code>. It cannot be changed later.
        </small>
        <label>
          Title
          <input value={form.title} onChange={(e) => set("title", e.target.value)} required maxLength={200} />
        </label>
        <label>
          Difficulty
          <select
            value={form.difficulty}
            onChange={(e) => set("difficulty", e.target.value as ProblemInput["difficulty"])}
          >
            <option value="easy">Easy</option>
            <option value="medium">Medium</option>
            <option value="hard">Hard</option>
          </select>
        </label>
        <label>
          Time limit (ms)
          <input
            type="number"
            min={100}
            max={10000}
            step={100}
            required
            value={Number.isNaN(form.time_limit_ms) ? "" : form.time_limit_ms}
            onChange={(e) => set("time_limit_ms", e.target.valueAsNumber)}
          />
        </label>
        <label>
          Memory limit (MB)
          <input
            type="number"
            min={16}
            max={512}
            required
            value={Number.isNaN(form.memory_limit_mb) ? "" : form.memory_limit_mb}
            onChange={(e) => set("memory_limit_mb", e.target.valueAsNumber)}
          />
        </label>
        <label className="checkbox">
          <input type="checkbox" checked={form.published} onChange={(e) => set("published", e.target.checked)} />
          Published (visible to everyone)
        </label>
      </div>

      <h2>Statement</h2>
      <div className="statement-editor">
        <label>
          Markdown
          <textarea
            value={form.statement_md}
            onChange={(e) => set("statement_md", e.target.value)}
            rows={16}
            maxLength={65536}
          />
        </label>
        <div aria-labelledby="preview-heading" role="region">
          <h3 id="preview-heading">Preview</h3>
          <Suspense fallback={<p role="status">Loading preview…</p>}>
            <Statement markdown={statement} />
          </Suspense>
        </div>
      </div>

      <h2>Checker</h2>
      <div className="fields">
        <label>
          Checker
          <select value={form.checker} onChange={(e) => set("checker", e.target.value as Checker)}>
            <option value="exact">Exact (line by line)</option>
            <option value="float">Float (numbers within a tolerance)</option>
            <option value="custom">Custom (Python program)</option>
          </select>
        </label>
        {form.checker === "float" && (
          <label>
            Tolerance
            <input
              type="number"
              min={0}
              step="any"
              placeholder="1e-6"
              value={tolerance}
              onChange={(e) => setTolerance(e.target.value)}
            />
          </label>
        )}
      </div>
      {form.checker === "custom" && (
        <label>
          Checker source (Python: reads the input, the expected and the actual output; prints AC or WA)
          <textarea
            className="code"
            value={form.checker_source ?? ""}
            onChange={(e) => set("checker_source", e.target.value)}
            rows={10}
            required
            spellCheck={false}
          />
        </label>
      )}

      <h2>Test cases</h2>
      <p>
        Sample test cases are shown on the problem page. Test data: {Math.ceil(bytes / 1024)} KB of{" "}
        {MAX_TEST_DATA_BYTES / 1024} KB, {form.test_cases.length} of {MAX_TEST_CASES} test cases.
      </p>
      {bytes > MAX_TEST_DATA_BYTES && <p className="error">The test data is larger than 4 MB.</p>}
      <label>
        Upload test case files (pairs NAME.in with NAME.out or NAME.ans)
        <input type="file" multiple accept=".in,.out,.ans" onChange={(e) => void onFiles(e)} />
      </label>
      {fileNotice && (
        <p className={fileNotice.error ? "error" : undefined} role="status">
          {fileNotice.text}
        </p>
      )}

      {form.test_cases.map((c, i) => (
        <fieldset key={i} className="test-case">
          <legend>Test case {i + 1}</legend>
          <label>
            Input
            <textarea
              className="code"
              value={c.input}
              onChange={(e) => setCase(i, { input: e.target.value })}
              rows={4}
              spellCheck={false}
            />
          </label>
          <label>
            Expected output
            <textarea
              className="code"
              value={c.output}
              onChange={(e) => setCase(i, { output: e.target.value })}
              rows={4}
              spellCheck={false}
            />
          </label>
          <label className="checkbox">
            <input type="checkbox" checked={c.sample} onChange={(e) => setCase(i, { sample: e.target.checked })} />
            Sample
          </label>
          <button
            type="button"
            onClick={() =>
              set(
                "test_cases",
                form.test_cases.filter((_, j) => j !== i),
              )
            }
            disabled={form.test_cases.length === 1}
          >
            Remove test case {i + 1}
          </button>
        </fieldset>
      ))}
      <button
        type="button"
        onClick={() => set("test_cases", [...form.test_cases, { input: "", output: "", sample: false }])}
        disabled={form.test_cases.length >= MAX_TEST_CASES}
      >
        Add test case
      </button>

      <div>
        <button type="submit" className="primary" disabled={save.isPending}>
          {save.isPending ? "Saving…" : slug ? "Save" : "Create problem"}
        </button>
      </div>
      {save.isError && <ErrorMessage error={save.error} />}
      {save.error instanceof ApiError && save.error.status === 409 && slug && (
        <p>Reload the page to see the latest version, then apply your changes again.</p>
      )}
    </form>
  );
}
