import path from "node:path";
import { fileURLToPath } from "node:url";
import { MatchersV3, PactV4 } from "@pact-foundation/pact";
import { type Api, createApi } from "../client";

export const PACT_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../../pacts");

/** The consumer / provider names; the Go provider test uses the same file name. */
export function newPact(): PactV4 {
  return new PactV4({
    consumer: "codearena-spa",
    provider: "codearena-api",
    dir: PACT_DIR,
    logLevel: "warn",
  });
}

/** The signed-in user of the contract. The provider test maps any bearer token to user alice. */
export const BEARER = "Bearer eyJhbGciOiJSUzI1NiJ9.alice.signature";

/** The request header that authenticates the user (the provider replaces the token). */
export const authorized = { Authorization: MatchersV3.regex("^Bearer .+$", BEARER) };

export function apiFor(baseUrl: string, signedIn = true): Api {
  const headers: Record<string, string> = signedIn ? { Authorization: BEARER } : {};
  return createApi(async () => headers, fetch, baseUrl);
}

/** RFC 3339 timestamp as Go writes it (fractional seconds optional). */
export function timestamp(example = "2026-11-13T10:00:00Z") {
  return MatchersV3.regex("^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(\\.\\d+)?(Z|[+-]\\d{2}:\\d{2})$", example);
}

export const SUBMISSION_ID = "0b6f4c3e-8d2a-4f1b-9c7e-5a3d2e1f0a9b";

export const problemDetails = {
  type: MatchersV3.string("about:blank"),
  title: MatchersV3.string("Not Found"),
  status: MatchersV3.integer(404),
  detail: MatchersV3.string("problem not found"),
};
