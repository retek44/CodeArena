# ADR-0022: Java, JavaScript and Go runners

- Status: accepted
- Date: 2026-11-15

## Context

PLAN.md lists Java, Node.js and Go after Python and C++. ADR-0013 already gives every
language its own image and Lambda function, so the question is how three managed or
compiled runtimes fit the sandbox (ADR-0003): a fixed address space (`RLIMIT_AS`), a small
process budget (`RLIMIT_NPROC`), a read-only root file system and a memory verdict based on
peak resident memory.

What we measured in the runner images:

- **Java (Corretto 21):** a JVM reserves far more address space than it uses (heap, code
  cache, class space, one malloc arena per thread) and starts about 15 threads. An empty
  program has about 40 MB resident.
- **Node.js 22:** about 50 MB resident for an empty program, a thread pool and V8 helper
  threads, large address space reservations.
- **Go 1.26:** the runtime reserves about 750 MB of address space at start (page
  summaries). With a limit between about 1100 and 1200 MB its first heap arena does not
  fit and every program dies with "runtime: out of memory (0 in use)"; below and above
  that window it works. The standard library is not shipped precompiled any more, so
  `go build` would compile `fmt` and friends into a writable build cache on every cold
  start, and a writable cache would be shared by all submissions of an execution
  environment.

## Decision

- **Language settings in the runner core** (`languages.py`): besides compile and run
  commands, a language can set a memory overhead (runtime memory that does not count
  against the problem's limit: Java 64 MB, JavaScript 48 MB), its address space overhead,
  the threads a run may start, the address space of the compile step, extra environment
  variables, and which compile outputs the runs need (only `*.class`, `main.js`, `main`).
  Run commands can use the problem's memory limit (`{memory_mb}`).
- **Java:** `javac`, then `java -Xmx<limit>m` with the serial GC, a small code cache and
  class space, no perf data file, `MALLOC_ARENA_MAX=2`. The class must be called `Main`.
  `OutOfMemoryError` is a memory verdict.
- **JavaScript:** `node --check` as the compile step (syntax errors are CE), then
  `node --max-old-space-size=<limit>`. A heap overflow ("JavaScript heap out of memory")
  is a memory verdict.
- **Go:** the image compiles the standard library once at build time and writes an
  `importcfg`. A submission is compiled and linked with the `compile` and `link` tools
  against it (`runners/go/build.sh`), with the same `GOEXPERIMENT` as the go command that
  built the library. No build cache is written at run time. The address space overhead is
  1536 MB, above the failure window for every limit from 16 to 512 MB.
- **Everything else is generic:** the worker reads `RUNNER_URL_<LANG>` /
  `RUNNER_FUNCTION_<LANG>`, Terraform creates one function per entry of
  `runner_languages`, CI builds and scans every image, the deploy hook knows an accepted
  A + B solution per language, and every language has golden solutions and sandbox
  scenarios (CPU loop, memory, network, clean environment afterwards).
- **The SPA** offers the languages with a starter each. JavaScript keeps Monaco's syntax
  highlighting but not the TypeScript language service (a 7 MB worker); the judge
  reports syntax errors anyway.

## Consequences

- Five runner images and functions. Images are larger (Go about 1.4 GB with the compiled
  standard library), which costs ECR storage and cold start time, not money per request.
- Java and JavaScript programs get their runtime's memory on top of the problem's limit,
  as most judges do; time limits are the same for every language, so a JVM or Node.js
  start (100–250 ms CPU) counts against them.
- The address space limit sits far above a Go program's memory limit (64 MB limit: 1600 MB
  address space), so `RLIMIT_AS` alone would let a Go program grow past the Lambda's 1 GB
  and end the whole invocation instead of returning MLE. The sandbox suite found this. The
  runner therefore watches resident memory while a program runs (every 10 ms, all
  processes of its session): a process above the memory limit, or all of them together
  above the Lambda's memory minus 160 MB for the runner, are stopped with a memory
  verdict.
- The Go address space window was measured on x86-64; the runners run on arm64 in AWS.
  CI builds and tests the arm64 images, so the golden and sandbox tests cover it.
- Not yet verified on AWS: the three new functions, their cold starts and the canary hook
  for them.
