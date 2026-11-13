import { type Api, type Difficulty, toApiError } from "./client";
import type { LanguageId } from "../languages";

// The API calls of the SPA, one function per endpoint. The hooks wrap them in TanStack
// Query; the Pact consumer tests (src/api/pact) call them against the Pact mock server,
// so the contract records exactly what the SPA sends and reads.

export const PAGE_SIZE = 20;

export async function listProblems(api: Api, cursor?: string, difficulty?: Difficulty) {
  const { data, error, response } = await api.GET("/api/problems", {
    params: { query: { limit: PAGE_SIZE, cursor, difficulty } },
  });
  if (!data) throw toApiError(error, response);
  return data;
}

export async function getProblem(api: Api, slug: string) {
  const { data, error, response } = await api.GET("/api/problems/{slug}", { params: { path: { slug } } });
  if (!data) throw toApiError(error, response);
  return data;
}

export async function getSubmission(api: Api, id: string) {
  const { data, error, response } = await api.GET("/api/submissions/{id}", { params: { path: { id } } });
  if (!data) throw toApiError(error, response);
  return data;
}

export async function listMySubmissions(api: Api, cursor?: string) {
  const { data, error, response } = await api.GET("/api/users/me/submissions", {
    params: { query: { limit: PAGE_SIZE, cursor } },
  });
  if (!data) throw toApiError(error, response);
  return data;
}

export interface SubmitInput {
  problem: string;
  language: LanguageId;
  source: string;
  /** Contest slug: a contest submission (counts for the leaderboard). */
  contest?: string;
  /** Generated once per click: retries of the same click reuse it (no duplicate submissions). */
  idempotencyKey: string;
}

export async function submit(api: Api, { problem, language, source, contest, idempotencyKey }: SubmitInput) {
  const { data, error, response } = await api.POST("/api/submissions", {
    params: { header: { "Idempotency-Key": idempotencyKey } },
    body: contest ? { problem, language, source, contest } : { problem, language, source },
  });
  if (!data) throw toApiError(error, response);
  return data;
}

export async function requestHint(api: Api, submissionId: string) {
  const { data, error, response } = await api.POST("/api/submissions/{id}/hint", {
    params: { path: { id: submissionId } },
  });
  if (!data) throw toApiError(error, response);
  return data;
}

export async function listContests(api: Api) {
  const { data, error, response } = await api.GET("/api/contests");
  if (!data) throw toApiError(error, response);
  return data.items;
}

export async function getContest(api: Api, slug: string) {
  const { data, error, response } = await api.GET("/api/contests/{slug}", { params: { path: { slug } } });
  if (!data) throw toApiError(error, response);
  return data;
}

export async function registerForContest(api: Api, slug: string) {
  const { error, response } = await api.POST("/api/contests/{slug}/register", { params: { path: { slug } } });
  if (!response.ok) throw toApiError(error, response);
}

export async function getLeaderboard(api: Api, slug: string) {
  const { data, error, response } = await api.GET("/api/contests/{slug}/leaderboard", {
    params: { path: { slug } },
  });
  if (!data) throw toApiError(error, response);
  return data;
}
