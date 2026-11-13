import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// Monaco needs a real browser (workers, canvas); component tests use a textarea.
// Playwright covers the real editor.
vi.mock("../components/CodeEditor", async () => {
  const { createElement } = await import("react");
  return {
    CodeEditor: ({ value, onChange, label }: { value: string; onChange: (v: string) => void; label: string }) =>
      createElement("textarea", { "aria-label": label, value, onChange: (e: { target: { value: string } }) => onChange(e.target.value) }),
  };
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});
