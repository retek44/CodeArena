import type { TestCase } from "../api/client";

const INPUT_EXTENSIONS = [".in"];
const OUTPUT_EXTENSIONS = [".out", ".ans"];

export interface TestFile {
  name: string;
  text: string;
}

function split(name: string): [base: string, extension: string] {
  const dot = name.lastIndexOf(".");
  return dot <= 0 ? [name, ""] : [name.slice(0, dot), name.slice(dot).toLowerCase()];
}

/**
 * Pairs NAME.in with NAME.out (or NAME.ans) into test cases, in natural order of NAME
 * (2 before 10). Files without a partner are returned in `unpaired`.
 */
export function pairTestFiles(files: TestFile[]): { cases: TestCase[]; unpaired: string[] } {
  const inputs = new Map<string, TestFile>();
  const outputs = new Map<string, TestFile>();
  const unpaired: string[] = [];
  for (const file of files) {
    const [base, extension] = split(file.name);
    if (INPUT_EXTENSIONS.includes(extension) && !inputs.has(base)) inputs.set(base, file);
    else if (OUTPUT_EXTENSIONS.includes(extension) && !outputs.has(base)) outputs.set(base, file);
    else unpaired.push(file.name);
  }

  const byName = [...inputs].sort(([a], [b]) => a.localeCompare(b, undefined, { numeric: true }));
  const cases: TestCase[] = [];
  for (const [base, input] of byName) {
    const output = outputs.get(base);
    if (!output) continue;
    cases.push({ input: input.text, output: output.text, sample: false });
    outputs.delete(base);
    inputs.delete(base);
  }
  unpaired.push(...[...inputs.values(), ...outputs.values()].map((f) => f.name));
  return { cases, unpaired: unpaired.sort() };
}

/** Bytes of the test data as the API counts them (UTF-8). */
export function testDataBytes(cases: TestCase[]): number {
  const encoder = new TextEncoder();
  return cases.reduce((sum, c) => sum + encoder.encode(c.input).length + encoder.encode(c.output).length, 0);
}
