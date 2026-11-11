"""Run one untrusted program with resource limits and classify how it ended. Linux only."""

from __future__ import annotations

import contextlib
import os
import resource
import select
import signal
import time
from dataclasses import dataclass
from typing import NoReturn

from . import linux
from .protocol import Status

STDERR_TAIL_BYTES = 4096
_MAX_FD_TO_CLOSE = 65536
_MAX_STACK_BYTES = 64 * 1024 * 1024
_MAX_OPEN_FILES = 64
_OOM_MARKERS = (b"MemoryError", b"std::bad_alloc", b"Cannot allocate memory", b"OutOfMemoryError")


@dataclass(frozen=True)
class ExecLimits:
    cpu_time_ms: int
    wall_time_ms: int
    memory_kb: int
    """Peak resident memory allowed (verdict threshold)."""
    address_space_bytes: int
    """RLIMIT_AS: hard cap on virtual memory, so a memory bomb fails fast."""
    output_bytes: int
    """RLIMIT_FSIZE: stdout and stderr cannot grow past this."""
    extra_tasks: int
    """Processes and threads the program may create (RLIMIT_NPROC above current usage)."""


@dataclass(frozen=True)
class ExecResult:
    status: Status
    cpu_time_ms: int
    wall_time_ms: int
    memory_kb: int
    exit_code: int | None
    signal: int | None
    stdout: bytes
    stderr: bytes


def execute(
    argv: tuple[str, ...],
    *,
    cwd: str,
    stdin_path: str,
    io_dir: str,
    limits: ExecLimits,
    env: dict[str, str],
) -> ExecResult:
    """Fork, apply limits, exec argv, wait with a wall-clock deadline, classify the outcome."""
    stdout_path = os.path.join(io_dir, "stdout")
    stderr_path = os.path.join(io_dir, "stderr")
    # RLIMIT_NPROC counts every task of the user, the runner included.
    nproc = linux.count_user_tasks(os.getuid()) + limits.extra_tasks

    with (
        open(stdin_path, "rb") as fin,
        open(stdout_path, "wb") as fout,
        open(stderr_path, "wb") as ferr,
    ):
        start = time.monotonic()
        pid = os.fork()
        if pid == 0:
            _exec_child(
                argv=argv,
                cwd=cwd,
                stdin_fd=fin.fileno(),
                stdout_fd=fout.fileno(),
                stderr_fd=ferr.fileno(),
                limits=limits,
                nproc=nproc,
                env=env,
            )

    timed_out = _wait_until(pid, limits.wall_time_ms / 1000)
    _, wait_status, usage = os.wait4(pid, 0)
    wall_time_ms = int((time.monotonic() - start) * 1000)
    # The program's session may still contain processes it started; stop them now.
    _kill_group(pid)

    stdout_size = os.path.getsize(stdout_path)
    with open(stdout_path, "rb") as f:
        stdout = f.read(limits.output_bytes)
    stderr = _read_tail(stderr_path, STDERR_TAIL_BYTES)

    exit_code: int | None = None
    term_signal: int | None = None
    if os.WIFSIGNALED(wait_status):
        term_signal = os.WTERMSIG(wait_status)
    else:
        exit_code = os.waitstatus_to_exitcode(wait_status)

    cpu_time_ms = int((usage.ru_utime + usage.ru_stime) * 1000)
    memory_kb = int(usage.ru_maxrss)  # kilobytes on Linux

    status = classify(
        limits=limits,
        timed_out=timed_out,
        cpu_time_ms=cpu_time_ms,
        memory_kb=memory_kb,
        stdout_size=stdout_size,
        exit_code=exit_code,
        term_signal=term_signal,
        stderr=stderr,
    )
    return ExecResult(
        status=status,
        cpu_time_ms=cpu_time_ms,
        wall_time_ms=wall_time_ms,
        memory_kb=memory_kb,
        exit_code=exit_code,
        signal=term_signal,
        stdout=stdout,
        stderr=stderr,
    )


def classify(
    *,
    limits: ExecLimits,
    timed_out: bool,
    cpu_time_ms: int,
    memory_kb: int,
    stdout_size: int,
    exit_code: int | None,
    term_signal: int | None,
    stderr: bytes,
) -> Status:
    """Decide the run status. Order matters: a killed program is TLE before it is RE."""
    if timed_out or cpu_time_ms > limits.cpu_time_ms or term_signal == signal.SIGXCPU:
        return Status.TIME_LIMIT
    # CPython ignores SIGXFSZ and gets EFBIG instead, so also check the file size.
    if term_signal == signal.SIGXFSZ or stdout_size >= limits.output_bytes:
        return Status.OUTPUT_LIMIT
    failed = term_signal is not None or exit_code != 0
    if memory_kb > limits.memory_kb or (failed and any(m in stderr for m in _OOM_MARKERS)):
        return Status.MEMORY_LIMIT
    if failed:
        return Status.RUNTIME_ERROR
    return Status.OK


def cpu_limit_seconds(cpu_time_ms: int) -> int:
    """RLIMIT_CPU has 1 s granularity. The soft limit sits above the verdict limit,
    so the measured CPU time decides TLE and the rlimit is only a safety net."""
    return -(-cpu_time_ms // 1000) + 1


def _wait_until(pid: int, timeout_s: float) -> bool:
    """Wait for the child to exit. Kill its process group on timeout. Returns True on timeout."""
    pidfd = os.pidfd_open(pid)
    try:
        ready, _, _ = select.select([pidfd], [], [], timeout_s)
    finally:
        os.close(pidfd)
    if ready:
        return False
    _kill_group(pid)
    return True


def _kill_group(pgid: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pgid, signal.SIGKILL)


def _read_tail(path: str, size: int) -> bytes:
    with open(path, "rb") as f:
        f.seek(0, os.SEEK_END)
        f.seek(max(0, f.tell() - size))
        return f.read()


def _exec_child(
    *,
    argv: tuple[str, ...],
    cwd: str,
    stdin_fd: int,
    stdout_fd: int,
    stderr_fd: int,
    limits: ExecLimits,
    nproc: int,
    env: dict[str, str],
) -> NoReturn:
    """Runs in the forked child. Never returns."""
    try:
        os.setsid()
        os.dup2(stdin_fd, 0)
        os.dup2(stdout_fd, 1)
        os.dup2(stderr_fd, 2)
        os.closerange(3, _MAX_FD_TO_CLOSE)
        os.chdir(cwd)
        os.umask(0o077)

        cpu_s = cpu_limit_seconds(limits.cpu_time_ms)
        resource.setrlimit(resource.RLIMIT_CPU, (cpu_s, cpu_s + 1))
        resource.setrlimit(resource.RLIMIT_AS, (limits.address_space_bytes, limits.address_space_bytes))
        resource.setrlimit(resource.RLIMIT_FSIZE, (limits.output_bytes, limits.output_bytes))
        resource.setrlimit(resource.RLIMIT_NOFILE, (_MAX_OPEN_FILES, _MAX_OPEN_FILES))
        resource.setrlimit(resource.RLIMIT_NPROC, (nproc, nproc))
        resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
        stack = min(_MAX_STACK_BYTES, limits.address_space_bytes)
        resource.setrlimit(resource.RLIMIT_STACK, (stack, stack))

        os.execve(argv[0], list(argv), env)
    except BaseException:
        try:
            os.write(2, b"runner: failed to start program\n")
        finally:
            os._exit(127)
    os._exit(127)  # unreachable; keeps type checkers happy
