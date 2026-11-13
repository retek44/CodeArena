# ADR-0013: One runner function per language; custom checkers run in the sandbox

- Status: accepted
- Date: 2026-11-13

## Context

Phase 3 adds C++ next to Python and problems with more than one correct answer. Both
touch the sandbox (ADR-0003): a compiler toolchain is large, and a custom checker is code
that must see the expected output.

## Decision

- **One image and one Lambda function per language** (`runners/<language>/Dockerfile`,
  `codearena-runner-<language>`). All images share the Python runner core; the image
  selects its languages with `CODEARENA_LANGUAGES`. The worker routes by language
  (`runner.ByLanguage` locally, `LambdaInvoker.Functions` in AWS).
- **C++:** `g++ -std=gnu++20 -O2 -static`. Static linking keeps the run independent of
  shared libraries; the binary runs from `/tmp`, which Lambda allows (Docker tmpfs needs
  `exec` locally). The compile step gets its own file-size limit (64 MB), because
  `RLIMIT_FSIZE` also covers the binary the linker writes.
- **Checkers:** `exact` and `float` run in the worker. A `custom` checker (Python, stored
  with the problem) runs as a *second* runner invocation with
  `{input, expected, output}` per test case and prints `AC` or `WA`. The submission's own
  invocation still never receives expected outputs. A broken checker is an error: the
  attempt is retried, then the submission gets IE.
- The output budget per test case shrinks with the number of test cases, so a runner
  response always fits the 6 MB Lambda response limit.

## Consequences

- Adding a language = a Dockerfile, a `languages.py` entry, the enum in the contracts,
  golden solutions, and the name in `local.runner_languages` (Terraform).
- Images stay small and toolchains do not mix; cold starts are per language.
- A custom checker costs one more invocation per judged submission.
- The OpenAPI response uses `x-extensible-enum` for the language, so adding a language is
  not a breaking change for clients.
