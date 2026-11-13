"""Language definitions. Each runner image supports the languages its toolchain provides.

The image selects them with CODEARENA_LANGUAGES (comma separated, default "python"):
one Lambda function per language keeps each image small and its toolchain minimal.
"""

from __future__ import annotations

import os
import sys
from dataclasses import dataclass

# Compile check for Python: report syntax errors as a compile error, not a runtime error.
_PY_SYNTAX_CHECK = """
import sys, traceback
try:
    compile(open(sys.argv[1], "rb").read(), sys.argv[1], "exec")
except SyntaxError as exc:
    sys.stderr.write("".join(traceback.format_exception_only(exc)))
    sys.exit(1)
"""


@dataclass(frozen=True)
class Language:
    name: str
    source_file: str
    run: tuple[str, ...]
    compile: tuple[str, ...] | None = None
    # Address space the toolchain needs on top of the user's memory limit.
    # RLIMIT_AS counts virtual memory, which is much larger than resident memory.
    address_space_overhead_mb: int = 64


LANGUAGES: dict[str, Language] = {
    "python": Language(
        name="python",
        source_file="main.py",
        # -I: isolated mode (no user site, no PYTHON* env vars); -S: no site import.
        run=(sys.executable, "-I", "-S", "main.py"),
        compile=(sys.executable, "-I", "-S", "-c", _PY_SYNTAX_CHECK, "main.py"),
        address_space_overhead_mb=96,
    ),
    "cpp": Language(
        name="cpp",
        source_file="main.cpp",
        run=("./main",),
        # Absolute paths: the sandbox calls execve without a PATH search. Static linking:
        # the program needs no shared libraries at run time, so the run
        # works the same with a minimal environment and a fresh working directory.
        compile=("/usr/bin/g++", "-std=gnu++20", "-O2", "-pipe", "-static", "-s", "-o", "main", "main.cpp"),
        address_space_overhead_mb=32,
    ),
}


def enabled_languages(env: dict[str, str] | None = None) -> dict[str, Language]:
    """Languages this image serves, from CODEARENA_LANGUAGES."""
    raw = (env if env is not None else os.environ).get("CODEARENA_LANGUAGES", "python")
    names = [name.strip() for name in raw.split(",") if name.strip()]
    unknown = sorted(set(names) - set(LANGUAGES))
    if unknown or not names:
        raise ValueError(f"CODEARENA_LANGUAGES must list languages from {sorted(LANGUAGES)}, got {raw!r}")
    return {name: LANGUAGES[name] for name in names}
