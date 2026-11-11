"""Small Linux process helpers (prctl and /proc). Linux only."""

from __future__ import annotations

import ctypes
import os

_PR_SET_DUMPABLE = 4
_PR_SET_CHILD_SUBREAPER = 36


def _prctl(option: int, value: int) -> None:
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(option, value, 0, 0, 0) != 0:
        errno = ctypes.get_errno()
        raise OSError(errno, f"prctl({option}, {value}) failed: {os.strerror(errno)}")


def harden_current_process() -> None:
    """Protect the runner from the programs it starts.

    - Not dumpable: /proc/<runner>/environ and /proc/<runner>/mem become unreadable
      for other processes of the same user, and ptrace is refused. The Lambda
      credentials live in the runner's environment.
    - Child subreaper: orphaned descendants (double fork, setsid) are re-parented to
      the runner instead of PID 1, so the runner can always find, kill and reap them.
    """
    _prctl(_PR_SET_DUMPABLE, 0)
    _prctl(_PR_SET_CHILD_SUBREAPER, 1)


def read_status(pid: int) -> dict[str, str]:
    """Parse /proc/<pid>/status. Raises OSError if the process is gone."""
    fields: dict[str, str] = {}
    with open(f"/proc/{pid}/status", encoding="utf-8", errors="replace") as f:
        for line in f:
            key, _, value = line.partition(":")
            fields[key] = value.strip()
    return fields


def list_pids() -> list[int]:
    return [int(name) for name in os.listdir("/proc") if name.isdigit()]


def ancestors(pid: int) -> set[int]:
    """PIDs of all ancestors of pid, including PID 1."""
    result: set[int] = set()
    current = pid
    while current > 1:
        try:
            parent = int(read_status(current).get("PPid", "0"))
        except (OSError, ValueError):
            break
        if parent <= 0 or parent in result:
            break
        result.add(parent)
        current = parent
    return result


def count_user_tasks(uid: int) -> int:
    """Number of tasks (threads) owned by uid. RLIMIT_NPROC counts these."""
    total = 0
    for pid in list_pids():
        try:
            status = read_status(pid)
        except OSError:
            continue
        if _real_uid(status) == uid:
            total += int(status.get("Threads", "1") or 1)
    return total


def _real_uid(status: dict[str, str]) -> int:
    uid_field = status.get("Uid", "")
    return int(uid_field.split()[0]) if uid_field else -1


def foreign_processes(uid: int) -> list[int]:
    """Live processes of uid that are neither the runner nor its ancestors.

    Zombies are ignored: they are already dead and are reaped by the runner,
    which is their subreaper.
    """
    me = os.getpid()
    protected = ancestors(me) | {me}
    result = []
    for pid in list_pids():
        if pid in protected:
            continue
        try:
            status = read_status(pid)
        except OSError:
            continue
        if _real_uid(status) != uid:
            continue
        if status.get("State", "").startswith(("Z", "X")):
            continue
        result.append(pid)
    return result
