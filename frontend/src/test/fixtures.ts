import type { Problem, Submission } from "../api/client";

export const aPlusB: Problem = {
  slug: "a-plus-b",
  title: "A + B",
  difficulty: "easy",
  tags: ["math"],
  statement_md: "Read two integers and print their sum.",
  time_limit_ms: 1000,
  memory_limit_mb: 64,
  samples: [{ input: "1 2\n", output: "3\n" }],
};

export function submission(overrides: Partial<Submission> = {}): Submission {
  return {
    id: "6b1f0b2e-1c2d-4e5f-8a9b-0c1d2e3f4a5b",
    problem: "a-plus-b",
    language: "python",
    status: "queued",
    verdict: null,
    runtime_ms: null,
    memory_kb: null,
    compile_output: null,
    created_at: "2026-11-13T10:00:00Z",
    judged_at: null,
    cases: [],
    ...overrides,
  };
}
