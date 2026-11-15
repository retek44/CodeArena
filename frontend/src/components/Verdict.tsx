import type { Submission } from "../api/client";
import { Flap, type FlapTone } from "./Flap";

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

function verdictTone(verdict: string): "ok" | "bad" | "muted" {
  return verdict === "AC" ? "ok" : verdict === "SKIPPED" ? "muted" : "bad";
}

const FLAP_TONES: Record<"ok" | "bad" | "muted", FlapTone> = { ok: "ok", bad: "bad", muted: "wait" };

/** Colour is never the only signal: the badge always shows the code and the full label. */
export function VerdictBadge({ verdict, flipOnMount = false }: { verdict: string; flipOnMount?: boolean }) {
  const tone = verdictTone(verdict);
  return (
    <Flap
      tone={FLAP_TONES[tone]}
      state={verdict}
      flipOnMount={flipOnMount}
      title={verdictLabel(verdict)}
      className={`badge badge-${tone}`}
    >
      <span className="badge-code">{verdict}</span> <span className="badge-label">{verdictLabel(verdict)}</span>
    </Flap>
  );
}

/** One cell for the whole life of a submission: it flips from queued to judging to the verdict. */
export function SubmissionStatus({ submission }: { submission: Submission }) {
  const { verdict, status } = submission;
  if (verdict) {
    const tone = verdictTone(verdict);
    return (
      <Flap tone={FLAP_TONES[tone]} state={verdict} title={verdictLabel(verdict)} className={`badge badge-${tone}`}>
        <span className="badge-code">{verdict}</span> <span className="badge-label">{verdictLabel(verdict)}</span>
      </Flap>
    );
  }
  const running = status === "running";
  return (
    <Flap tone={running ? "run" : "wait"} state={status} className="badge badge-muted" role="status">
      {running ? "Judging…" : "In queue…"}
    </Flap>
  );
}
