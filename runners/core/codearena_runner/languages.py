"""Language definitions. Each runner image supports the languages its toolchain provides."""

from __future__ import annotations

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
}
