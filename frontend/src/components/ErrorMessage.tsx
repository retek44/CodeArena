import { ApiError } from "../api/client";

function formatWait(seconds: number): string {
  if (seconds < 120) return `${seconds} seconds`;
  return `${Math.ceil(seconds / 60)} minutes`;
}

export function errorText(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 429 && error.retryAfterSeconds) {
      return `Too many requests. Try again in ${formatWait(error.retryAfterSeconds)}.`;
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
