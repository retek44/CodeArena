import { ApiError } from "../api/client";

export function errorText(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 429 && error.retryAfterSeconds) {
      return `Too many submissions. Try again in ${error.retryAfterSeconds} seconds.`;
    }
    if (error.status === 401) return "Please sign in again.";
    return error.message;
  }
  return error instanceof Error ? error.message : "Something went wrong.";
}

export function ErrorMessage({ error }: { error: unknown }) {
  return (
    <p className="error" role="alert">
      {errorText(error)}
    </p>
  );
}
