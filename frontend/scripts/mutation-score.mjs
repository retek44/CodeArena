// Prints the Stryker mutation score as Markdown (nightly job summary).
import { readFileSync } from "node:fs";

const report = JSON.parse(readFileSync("reports/mutation/mutation.json", "utf8"));
const mutants = Object.values(report.files).flatMap((file) => file.mutants);
const count = (status) => mutants.filter((m) => m.status === status).length;
const killed = count("Killed") + count("Timeout");
const valid = mutants.length - count("CompileError") - count("RuntimeError") - count("Ignored");
const score = valid === 0 ? 0 : (100 * killed) / valid;
console.log(
  `### TypeScript mutation score (Stryker): ${score.toFixed(1)} % ` +
    `(${killed}/${valid} killed, ${count("Survived")} survived, ${count("NoCoverage")} not covered)`,
);
