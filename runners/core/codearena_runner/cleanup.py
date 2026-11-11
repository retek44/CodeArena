"""Return the execution environment to a clean state between runs. Linux only.

Lambda freezes and reuses execution environments. Anything a submission leaves behind
(background processes, files) would otherwise be visible to the next submission.
"""

from __future__ import annotations

import contextlib
import os
import shutil
import signal
import stat
import time
from collections.abc import Iterable

from . import linux

_KILL_ROUNDS = 20
_ROUND_PAUSE_S = 0.01


def reset_sandbox(uid: int, scratch_dirs: Iterable[str]) -> bool:
    """Kill leftover processes and empty the scratch directories.

    Returns False if processes survive; the caller must then recycle the
    execution environment.
    """
    clean = kill_foreign_processes(uid)
    for path in scratch_dirs:
        empty_dir(path)
    return clean


def kill_foreign_processes(uid: int) -> bool:
    for _ in range(_KILL_ROUNDS):
        reap_children()
        pids = linux.foreign_processes(uid)
        if not pids:
            return True
        for pid in pids:
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.kill(pid, signal.SIGKILL)
        time.sleep(_ROUND_PAUSE_S)
    reap_children()
    return not linux.foreign_processes(uid)


def reap_children() -> None:
    """Collect exit statuses of dead children (the runner is their subreaper)."""
    while True:
        try:
            pid, _ = os.waitpid(-1, os.WNOHANG)
        except ChildProcessError:
            return
        if pid == 0:
            return


def empty_dir(path: str) -> None:
    """Delete everything inside path, including entries the program made read-only."""
    if not os.path.isdir(path):
        return
    for entry in os.scandir(path):
        try:
            if entry.is_dir(follow_symlinks=False):
                _unlock_tree(entry.path)
                shutil.rmtree(entry.path, ignore_errors=True)
            else:
                os.unlink(entry.path)
        except OSError:
            # Whatever survives is removed by the next reset; it cannot affect the result.
            pass


def _unlock_tree(root: str) -> None:
    """Give the owner full access to every directory under root, so rmtree can descend."""
    stack = [root]
    while stack:
        current = stack.pop()
        try:
            os.chmod(current, stat.S_IRWXU)
            entries = list(os.scandir(current))
        except OSError:
            continue
        stack.extend(e.path for e in entries if e.is_dir(follow_symlinks=False))
