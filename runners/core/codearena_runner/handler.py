"""Lambda entry point of the runner.

Flow per invocation:
  1. validate the event (protocol v1)
  2. compile (or syntax-check) the source; keep the artifacts in memory
  3. for each test case: reset the sandbox, write fresh artifacts and input, run, collect
  4. reset the sandbox; if processes survive, exit the process so Lambda starts a
     new execution environment

The submission never sees expected outputs, other test cases or the runner's environment.
"""

from __future__ import annotations

import fnmatch
import json
import os
import sys
import tempfile
import time
from collections.abc import Callable
from typing import Any

from . import cleanup, linux
from .languages import Language, enabled_languages
from .protocol import (
    CaseResult,
    CompileResult,
    CompileStatus,
    Limits,
    Request,
    Response,
    Status,
    parse_request,
)
from .sandbox import ExecLimits, ExecResult, execute

SCRATCH_DIRS = ("/tmp", "/dev/shm")
_SCRATCH_ROOT = "/tmp"
_MB = 1024 * 1024
_SAFETY_MARGIN_MS = 2000
_COMPILE_EXTRA_TASKS = 64
_COMPILE_MEMORY_MB = 512
# Memory of the function the runner itself needs (Python, request, results). The program's
# processes together get the rest (ExecLimits.session_memory_kb).
_RUNNER_RESERVE_MB = 160
# RLIMIT_FSIZE of the compile step: it covers the binary the compiler writes, not only
# the diagnostics (a statically linked C++ program is a few MB).
_COMPILE_FILE_BYTES = 64 * _MB
_COMPILE_DIAGNOSTICS_CHARS = 8192

Artifacts = dict[str, tuple[bytes, int]]

_HARDENING_REFUSED = linux.harden_current_process()  # logged at the end of the module
_UID = os.getuid()
LANGUAGES = enabled_languages()


def handler(event: Any, context: Any) -> dict[str, Any]:
    request = parse_request(event, frozenset(LANGUAGES))
    remaining_ms = _remaining_time(context)
    started = time.monotonic()
    try:
        response = judge(request, remaining_ms)
    finally:
        _reset_or_recycle()
    _log(
        "judged",
        submission_id=request.submission_id,
        language=request.language,
        compile=str(response.compile.status),
        statuses=[str(r.status) for r in response.results],
        truncated=response.truncated,
        duration_ms=int((time.monotonic() - started) * 1000),
    )
    return response.to_dict()


def judge(request: Request, remaining_ms: Callable[[], int]) -> Response:
    language = LANGUAGES[request.language]
    _reset_or_recycle()

    compile_result, artifacts = _compile(language, request)
    response = Response(submission_id=request.submission_id, compile=compile_result)
    if compile_result.status != CompileStatus.OK:
        return response

    limits = _run_limits(request.limits, language)
    failed = False
    for case in request.test_cases:
        if failed and request.stop_on_failure:
            response.results.append(CaseResult(id=case.id, status=Status.SKIPPED))
            continue
        if remaining_ms() < limits.wall_time_ms + _SAFETY_MARGIN_MS:
            response.results.append(CaseResult(id=case.id, status=Status.SKIPPED))
            response.truncated = True
            continue

        _reset_or_recycle()
        workdir, io_dir = _fresh_dirs()
        _materialize(artifacts, workdir)
        stdin_path = os.path.join(io_dir, "stdin")
        with open(stdin_path, "w", encoding="utf-8", newline="") as f:
            f.write(case.input)

        result = execute(
            language.run_command(request.limits.memory_mb),
            cwd=workdir,
            stdin_path=stdin_path,
            io_dir=io_dir,
            limits=limits,
            env=_child_env(workdir, language),
        )
        response.results.append(_case_result(case.id, result))
        failed = failed or result.status != Status.OK

    return response


def _compile(language: Language, request: Request) -> tuple[CompileResult, Artifacts]:
    """Compile in a scratch directory and return the artifacts as bytes.

    Artifacts stay in runner memory and are written fresh for every test case,
    so a run cannot modify the program that the next test case executes.
    """
    workdir, io_dir = _fresh_dirs()
    source_path = os.path.join(workdir, language.source_file)
    with open(source_path, "w", encoding="utf-8", newline="") as f:
        # Writing untrusted source to the sandbox scratch directory is the runner's job (not a web request).
        f.write(request.source)  # nosemgrep: python.django.security.injection.request-data-write.request-data-write

    if language.compile is not None:
        stdin_path = os.path.join(io_dir, "stdin")
        open(stdin_path, "wb").close()
        result = execute(
            language.compile,
            cwd=workdir,
            stdin_path=stdin_path,
            io_dir=io_dir,
            limits=_compile_limits(request.limits, language),
            env=_child_env(workdir, language),
        )
        diagnostics = (result.stdout + result.stderr).decode("utf-8", errors="replace")[-_COMPILE_DIAGNOSTICS_CHARS:]
        if result.status == Status.TIME_LIMIT:
            return CompileResult(CompileStatus.TIME_LIMIT, diagnostics), {}
        if result.status != Status.OK:
            return CompileResult(CompileStatus.ERROR, diagnostics), {}

    artifacts: Artifacts = {}
    for root, _, files in os.walk(workdir):
        for name in files:
            path = os.path.join(root, name)
            rel_path = os.path.relpath(path, workdir)
            if language.artifacts is not None and not any(fnmatch.fnmatch(rel_path, p) for p in language.artifacts):
                continue
            with open(path, "rb") as f:
                artifacts[rel_path] = (f.read(), os.stat(path).st_mode & 0o700)
    return CompileResult(CompileStatus.OK), artifacts


def _run_limits(limits: Limits, language: Language) -> ExecLimits:
    memory_mb = limits.memory_mb + language.memory_overhead_mb
    return ExecLimits(
        cpu_time_ms=limits.time_ms,
        # Wall clock catches programs that sleep or block instead of using CPU.
        wall_time_ms=limits.time_ms * 2 + 1000,
        memory_kb=memory_mb * 1024,
        address_space_bytes=(memory_mb + language.address_space_overhead_mb) * _MB,
        output_bytes=limits.output_kb * 1024,
        extra_tasks=language.run_tasks,
        session_memory_kb=session_memory_kb(),
    )


def _compile_limits(limits: Limits, language: Language) -> ExecLimits:
    return ExecLimits(
        cpu_time_ms=limits.compile_time_ms,
        wall_time_ms=limits.compile_time_ms * 2,
        memory_kb=_COMPILE_MEMORY_MB * 1024,
        address_space_bytes=language.compile_address_space_mb * _MB,
        output_bytes=_COMPILE_FILE_BYTES,
        extra_tasks=_COMPILE_EXTRA_TASKS,
        session_memory_kb=session_memory_kb(),
    )


def session_memory_kb() -> int | None:
    """Memory the program's processes may use together: the Lambda's memory minus what the
    runner needs. None outside Lambda (no memory size known)."""
    size = os.environ.get("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", "")
    return (int(size) - _RUNNER_RESERVE_MB) * 1024 if size.isdigit() else None


def _case_result(case_id: str, result: ExecResult) -> CaseResult:
    return CaseResult(
        id=case_id,
        status=result.status,
        cpu_time_ms=result.cpu_time_ms,
        wall_time_ms=result.wall_time_ms,
        memory_kb=result.memory_kb,
        exit_code=result.exit_code,
        signal=result.signal,
        stdout=result.stdout.decode("utf-8", errors="replace"),
        stderr=result.stderr.decode("utf-8", errors="replace"),
    )


def _child_env(workdir: str, language: Language) -> dict[str, str]:
    # A fixed, minimal environment plus the language's own settings. Nothing from the
    # runner's environment (AWS credentials, Lambda variables) reaches the program.
    return {
        "PATH": "/var/lang/bin:/usr/local/bin:/usr/bin:/bin",
        "HOME": workdir,
        "TMPDIR": workdir,
        "LANG": "C.UTF-8",
        **dict(language.env),
    }


def _fresh_dirs() -> tuple[str, str]:
    return tempfile.mkdtemp(prefix="run-", dir=_SCRATCH_ROOT), tempfile.mkdtemp(prefix="io-", dir=_SCRATCH_ROOT)


def _materialize(artifacts: Artifacts, workdir: str) -> None:
    for rel_path, (content, mode) in artifacts.items():
        path = os.path.join(workdir, rel_path)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as f:
            f.write(content)
        os.chmod(path, mode | 0o400)


def _reset_or_recycle() -> None:
    if cleanup.reset_sandbox(_UID, SCRATCH_DIRS):
        return
    _log("sandbox_not_clean", action="recycle_execution_environment")
    sys.stdout.flush()
    # Lambda reports this invocation as failed and starts a fresh execution environment.
    os._exit(1)


def _remaining_time(context: Any) -> Callable[[], int]:
    getter = getattr(context, "get_remaining_time_in_millis", None)
    if callable(getter):
        return lambda: int(getter())
    return lambda: 10**9


def _log(event: str, **fields: Any) -> None:
    print(json.dumps({"event": event, **fields}), flush=True)


if _HARDENING_REFUSED:
    # Expected in Lambda (seccomp). The entrypoint already removed the AWS credentials.
    _log("hardening_refused", protections=_HARDENING_REFUSED)
