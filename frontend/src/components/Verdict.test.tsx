import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { errorText } from "./ErrorMessage";
import { ApiError } from "../api/client";
import { VerdictBadge, verdictLabel } from "./Verdict";

describe("VerdictBadge", () => {
  it.each([
    ["AC", "Accepted", "badge-ok"],
    ["WA", "Wrong answer", "badge-bad"],
    ["TLE", "Time limit exceeded", "badge-bad"],
    ["SKIPPED", "Skipped", "badge-muted"],
  ])("%s shows its label and tone", (verdict, label, tone) => {
    render(<VerdictBadge verdict={verdict} />);
    const badge = screen.getByTitle(label);
    expect(badge).toHaveTextContent(`${verdict} ${label}`);
    expect(badge).toHaveClass(tone);
  });

  it("falls back to the code for unknown verdicts", () => {
    expect(verdictLabel("XYZ")).toBe("XYZ");
  });
});

describe("errorText", () => {
  it.each([
    [new ApiError(429, "limited", 30), "Too many submissions. Try again in 30 seconds."],
    [new ApiError(401, "unauthenticated"), "Please sign in again."],
    [new ApiError(404, "problem not found"), "problem not found"],
    [new Error("network down"), "network down"],
    ["weird", "Something went wrong."],
  ])("%s", (error, text) => {
    expect(errorText(error)).toBe(text);
  });
});
