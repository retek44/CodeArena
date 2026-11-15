import { describe, expect, it } from "vitest";
import { pairTestFiles, testDataBytes } from "./testFiles";

const file = (name: string, text = name) => ({ name, text });

describe("pairTestFiles", () => {
  it("pairs .in with .out and .ans in natural order", () => {
    const { cases, unpaired } = pairTestFiles([
      file("10.in"),
      file("2.out"),
      file("10.ans"),
      file("2.in"),
      file("1.IN"),
      file("1.out"),
    ]);

    expect(cases.map((c) => [c.input, c.output])).toEqual([
      ["1.IN", "1.out"],
      ["2.in", "2.out"],
      ["10.in", "10.ans"],
    ]);
    expect(cases.every((c) => !c.sample)).toBe(true);
    expect(unpaired).toEqual([]);
  });

  it("reports files without a partner or with another extension", () => {
    const { cases, unpaired } = pairTestFiles([
      file("a.in"),
      file("b.out"),
      file("c.in"),
      file("c.out"),
      file("notes.txt"),
      file("README"),
    ]);

    expect(cases).toHaveLength(1);
    expect(unpaired).toEqual(["README", "a.in", "b.out", "notes.txt"]);
  });
});

describe("testDataBytes", () => {
  it("counts UTF-8 bytes of inputs and outputs", () => {
    expect(testDataBytes([{ input: "é\n", output: "1\n", sample: false }])).toBe(5);
  });
});
