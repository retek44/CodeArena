import { useState, type ReactNode } from "react";

export type FlapTone = "ok" | "bad" | "run" | "wait" | "plain";

/**
 * A split-flap cell for state: verdicts, phases, limits.
 *
 * The flip is the board's one motion. It plays when the cell's `state` changes while the
 * page is open (queued, running, a verdict) or, with `flipOnMount`, when a cell arrives
 * after the page loaded (a new test case result). Cells present at load stand still, so
 * the board is readable at once; reduced motion turns the flip off in CSS.
 */
export function Flap({
  children,
  tone = "plain",
  state,
  flipOnMount = false,
  title,
  className = "",
  role,
}: {
  children: ReactNode;
  tone?: FlapTone;
  /** Identity of what the cell shows; a change flips the cell. */
  state?: string;
  flipOnMount?: boolean;
  title?: string;
  className?: string;
  role?: string;
}) {
  const [initial] = useState(state);
  const changed = state !== initial;
  const flip = flipOnMount || changed;
  return (
    <span
      // A new key restarts the flip animation on every change.
      key={changed ? state : "initial"}
      className={`flap flap-${tone}${flip ? " flap-flip" : ""} ${className}`.trim()}
      title={title}
      role={role}
    >
      {children}
    </span>
  );
}
