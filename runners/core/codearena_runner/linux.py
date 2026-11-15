"""Small Linux process helpers (prctl and /proc). Linux only."""

from __future__ import annotations

import ctypes
import os
from errno import EPERM

_PR_SET_DUMPABLE = 4
_PR_SET_CHILD_SUBREAPER = 36


def _prctl(option: int, value: int) -> bool:
    """True if set. False if the environment refuses the option (EPERM from a seccomp
    filter, as in Lambda). Any other error is a bug and raises."""
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(option, value, 0, 0, 0) == 0:
        return True
    errno = ctypes.get_errno()
    if errno == EPERM:
        return False
    raise OSError(errno, f"prctl({option}, {value}) failed: {os.strerror(errno)}")


def harden_current_process() -> list[str]:
    """Protect the runner from the programs it starts, where the environment allows it.

    - Not dumpable: /proc/<runner>/environ and /proc/<runner>/mem become unreadable
      for other processes of the same user, and ptrace is refused.
    - Child subreaper: orphaned descendants (double fork, setsid) are re-parented to
      the runner instead of PID 1, so the runner can reap them. Cleanup finds leftovers
      by user ID, so it still kills them without this.

    Lambda's seccomp filter refuses PR_SET_DUMPABLE. That is why the image entrypoint
    (runners/core/entrypoint.sh) removes the AWS credentials before the runtime starts:
    no runner process holds a secret that /proc could show.

    Returns the protections the environment refused.
    """
    refused: list[str] = []
    if not _prctl(_PR_SET_DUMPABLE, 0):
        refused.append("not_dumpable")
    if not _prctl(_PR_SET_CHILD_SUBREAPER, 1):
        refused.append("child_subreaper")
    return refused


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
