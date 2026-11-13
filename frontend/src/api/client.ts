import createClient, { type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

// Types generated from contracts/openapi.yaml (npm run api:types). The CI fails when
// the file is out of date, so the frontend always matches the API contract.
export type Schemas = components["schemas"];
export type Problem = Schemas["Problem"];
export type ProblemSummary = Schemas["ProblemSummary"];
export type Submission = Schemas["Submission"];
export type Verdict = Schemas["Verdict"];
export type Difficulty = Schemas["Difficulty"];
export type Hint = Schemas["Hint"];

/** Returns the headers that authenticate a request (empty when signed out). */
export type AuthHeaders = () => Promise<Record<string, string>>;

export function createApi(
  authHeaders: AuthHeaders,
  fetchImpl?: typeof fetch,
  // An absolute base URL: relative URLs fail outside the browser (tests).
  baseUrl = globalThis.location?.origin ?? "",
) {
  const client = createClient<paths>({
    baseUrl,
    fetch: fetchImpl,
  });
  const auth: Middleware = {
    async onRequest({ request }) {
      for (const [name, value] of Object.entries(await authHeaders())) {
        request.headers.set(name, value);
      }
      return request;
    },
  };
  client.use(auth);
  return client;
}

export type Api = ReturnType<typeof createApi>;

/** An RFC 9457 error response from the API. */
export class ApiError extends Error {
  readonly status: number;
  readonly retryAfterSeconds?: number;

  constructor(status: number, detail: string, retryAfterSeconds?: number) {
    super(detail);
    this.name = "ApiError";
    this.status = status;
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

export function toApiError(body: unknown, response: Response): ApiError {
  const problem = body as Partial<Schemas["ProblemDetails"]> | undefined;
  const detail = problem?.detail || problem?.title || response.statusText || "Request failed";
  const retryAfter = Number(response.headers.get("Retry-After"));
  return new ApiError(response.status, detail, Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : undefined);
}
