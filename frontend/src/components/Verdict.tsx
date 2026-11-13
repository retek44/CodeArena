import type { Submission } from "../api/client";

const LABELS: Record<string, string> = {
  AC: "Accepted",
  WA: "Wrong answer",
  TLE: "Time limit exceeded",
  MLE: "Memory limit exceeded",
  OLE: "Output limit exceeded",
  RE: "Runtime error",
  CE: "Compile error",
  IE: "Internal error",
  SKIPPED: "Skipped",
};

export function verdictLabel(verdict: string): string {
  return LABELS[verdict] ?? verdict;
}

/** Colour is never the only signal: the badge always shows the code and the full label. */
export function VerdictBadge({ verdict }: { verdict: string }) {
  const tone = verdict === "AC" ? "ok" : verdict === "SKIPPED" ? "muted" : "bad";
  return (
    <span className={`badge badge-${tone}`} title={verdictLabel(verdict)}>
      {verdict} <span className="badge-label">{verdictLabel(verdict)}</span>
    </span>
  );
}

export function SubmissionStatus({ submission }: { submission: Submission }) {
  if (submission.verdict) return <VerdictBadge verdict={submission.verdict} />;
  return (
    <span className="badge badge-muted" role="status">
      {submission.status === "running" ? "Judging…" : "In queue…"}
    </span>
  );
}
