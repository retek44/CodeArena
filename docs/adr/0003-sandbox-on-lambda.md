# ADR-0003: Run untrusted code in Lambda

- Status: accepted
- Date: 2026-10-12

## Context

The judge runs code that anyone can submit. The code must not reach the network, other
users' data, the AWS account or the host, and it must not run forever or use unlimited memory.

Fargate does not allow privileged containers, Docker-in-Docker or `CAP_SYS_ADMIN`, so tools
such as nsjail or a nested container runtime do not work inside a Fargate task.

## Decision

Run each submission in a **per-language Lambda function** (container image):

- Isolation: Lambda runs every execution environment in its own Firecracker microVM.
- No network: the function is attached to isolated subnets with no route to the internet
  and no VPC endpoints.
- No AWS access: the execution role allows only logs, VPC network interfaces and X-Ray.
  The security group of the function has no rules at all.
- Limits: the runner starts the user program as a subprocess with a non-root user and
  `prlimit` (CPU time, address space, number of processes, file size). Compile and run
  have separate time limits. The Lambda timeout is the last safety net.
- **Execution environment reuse:** Lambda freezes and reuses the environment, so a
  background process or a file in `/tmp` could survive into the next submission. After
  each run the runner kills the whole process group and wipes `/tmp`. If any process is
  still alive, the runner calls `os._exit(1)`, which forces Lambda to start a new environment.

The judge worker (Fargate) only orchestrates: it fetches test cases, invokes the runner,
compares outputs and stores the verdict.

## Consequences

- Strong isolation without running our own VM fleet.
- Cold starts for container images (a few hundred milliseconds to seconds). Provisioned
  concurrency is possible for contests, at extra cost.
- Max 15 minutes and 10 GB memory per invocation – far above judge limits.
- Unit tests cover limit classification; golden tests cover the verdicts end to end. A
  dedicated hardening suite (`tests/sandbox-security`) is still open.

## Implementation notes (phase 1)

- The runner process is not dumpable (`PR_SET_DUMPABLE`) and is a child subreaper
  (`PR_SET_CHILD_SUBREAPER`), so it can always find and reap everything a run started.
- The program gets a fixed, minimal environment (`PATH`, `HOME`, `TMPDIR`, `LANG`); nothing
  from the runner's environment is passed on.
- Compile artifacts stay in runner memory. Every test case gets a fresh working directory
  with fresh artifacts and only its own input; expected outputs never reach the runner.
- Limits per run: `RLIMIT_CPU` (safety net above the verdict limit), `RLIMIT_AS`,
  `RLIMIT_FSIZE` (output limit), `RLIMIT_NPROC`, `RLIMIT_NOFILE`, no core dumps; a wall-clock
  deadline through `pidfd`. CPU time and peak memory come from `wait4` resource usage.
- Before and after every test case the runner resets the sandbox. If the reset fails,
  it exits and Lambda starts a new execution environment.
- Locally and in CI the container runs like Lambda: non-root user, read-only root file
  system, writable `/tmp` only.
- Limitation: Lambda runs the runner and the program as the same user, so a program can
  disturb the runner process itself. The worker treats a failed invocation as a retry and
  then as a runtime error. Seccomp filtering is a possible later step.

## Alternatives

- EC2 workers with gVisor or Firecracker: full control, but more operations work and cost.
- One Fargate task per submission: strong isolation, but 30–60 s start time per submission.
