import { rmSync } from "node:fs";
import { PACT_DIR } from "./provider";

// Pact merges new interactions into an existing file: start from an empty directory so
// removed interactions disappear from the contract.
export function setup() {
  rmSync(PACT_DIR, { recursive: true, force: true });
}
