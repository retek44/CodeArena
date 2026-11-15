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


# Placeholder in run commands: the problem's memory limit in MB (JVM heap, V8 old space).
MEMORY_MB = "{memory_mb}"


@dataclass(frozen=True)
class Language:
    name: str
    source_file: str
    run: tuple[str, ...]
    compile: tuple[str, ...] | None = None
    # Address space the toolchain needs on top of the user's memory limit.
    # RLIMIT_AS counts virtual memory, which is much larger than resident memory.
    address_space_overhead_mb: int = 64
    # Resident memory of the runtime itself (JVM, V8) that does not count against the
    # problem's limit: the memory verdict allows the limit plus this much.
    memory_overhead_mb: int = 0
    # Files of the compile step that the runs need (glob patterns); None keeps them all.
    artifacts: tuple[str, ...] | None = None
    # Processes and threads one run may create (RLIMIT_NPROC above current usage).
    # Managed runtimes start helper threads (GC, JIT compiler, I/O pool).
    run_tasks: int = 16
    # Address space of the compile step (compilers that are themselves a JVM need more).
    compile_address_space_mb: int = 1024
    # Extra environment variables of compile and run (on top of the fixed minimal one).
    env: tuple[tuple[str, str], ...] = ()

    def run_command(self, memory_mb: int) -> tuple[str, ...]:
        return tuple(arg.replace(MEMORY_MB, str(memory_mb)) for arg in self.run)


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
        artifacts=("main",),
    ),
    "java": Language(
        name="java",
        source_file="Main.java",
        # Heap = the problem's memory limit. Serial GC: one GC thread, small footprint.
        # No perf data file in /tmp. Small reserved code cache and class space keep the
        # virtual address space (RLIMIT_AS) low.
        run=(
            "/usr/bin/java",
            f"-Xmx{MEMORY_MB}m",
            "-Xss64m",
            "-XX:+UseSerialGC",
            "-XX:ReservedCodeCacheSize=48m",
            "-XX:CompressedClassSpaceSize=32m",
            "-XX:MaxMetaspaceSize=96m",
            "-XX:-UsePerfData",
            "-Xshare:auto",
            "-cp",
            ".",
            "Main",
        ),
        compile=(
            "/usr/bin/javac",
            "-J-Xmx256m",
            "-J-XX:+UseSerialGC",
            "-J-XX:TieredStopAtLevel=1",
            "-J-XX:ReservedCodeCacheSize=48m",
            "-J-XX:CompressedClassSpaceSize=32m",
            "-J-XX:-UsePerfData",
            "-encoding",
            "UTF-8",
            "-nowarn",
            "Main.java",
        ),
        address_space_overhead_mb=1024,
        memory_overhead_mb=64,
        artifacts=("*.class",),
        run_tasks=48,
        compile_address_space_mb=2048,
        env=(("MALLOC_ARENA_MAX", "2"),),
    ),
    "javascript": Language(
        name="javascript",
        source_file="main.js",
        # One GC thread: CPU time stays the program's own, and fewer threads to budget.
        run=(
            "/usr/local/bin/codearena-node",
            f"--max-old-space-size={MEMORY_MB}",
            "--single-threaded-gc",
            "--stack-size=65500",
            "main.js",
        ),
        compile=("/usr/local/bin/codearena-node", "--check", "main.js"),
        address_space_overhead_mb=1024,
        memory_overhead_mb=48,
        artifacts=("main.js",),
        run_tasks=32,
        compile_address_space_mb=2048,
        env=(("MALLOC_ARENA_MAX", "2"), ("UV_THREADPOOL_SIZE", "1")),
    ),
    "go": Language(
        name="go",
        source_file="main.go",
        run=("./main",),
        # Compile and link against the standard library built into the image (runners/go).
        compile=("/opt/codearena-go/build.sh",),
        # The Go runtime reserves about 750 MB of address space at start (page summaries),
        # and with a limit between about 1100 and 1200 MB its first heap arena does not fit
        # (runtime: out of memory, 0 in use). 1536 keeps every limit (16-512 MB) above that.
        address_space_overhead_mb=1536,
        artifacts=("main",),
        env=(("GOMAXPROCS", "2"),),
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
