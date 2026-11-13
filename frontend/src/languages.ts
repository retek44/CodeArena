import type { components } from "./api/schema";

export type LanguageId = components["schemas"]["Language"];

export interface LanguageInfo {
  id: LanguageId;
  /** Shown in the language picker. */
  label: string;
  /** Monaco language id (syntax highlighting). */
  monaco: string;
  starter: string;
}

/** Languages a submission can use; each one has its own runner (Lambda function). */
export const LANGUAGES: readonly LanguageInfo[] = [
  {
    id: "python",
    label: "Python 3.13",
    monaco: "python",
    starter: "# Read from standard input, write to standard output.\n",
  },
  {
    id: "cpp",
    label: "C++20 (GCC)",
    monaco: "cpp",
    starter: "#include <iostream>\n\nint main() {\n    // Read from standard input, write to standard output.\n}\n",
  },
];

export const DEFAULT_LANGUAGE: LanguageId = "python";

export function languageInfo(id: LanguageId): LanguageInfo {
  const info = LANGUAGES.find((l) => l.id === id);
  if (!info) throw new Error(`unknown language ${id}`);
  return info;
}

/** Display name of a submission's language; unknown values (extensible enum) are shown as they are. */
export function languageLabel(id: string): string {
  return LANGUAGES.find((l) => l.id === id)?.label ?? id;
}
