import signal
import sys
from pathlib import Path

import pytest

if not sys.platform.startswith("linux"):
    pytest.skip("sandbox needs Linux", allow_module_level=True)

from codearena_runner.protocol import Status
from codearena_runner.sandbox import ExecLimits, ExecResult, classify, cpu_limit_seconds, execute

MB = 1024 * 1024
LIMITS = ExecLimits(
    cpu_time_ms=1000,
    wall_time_ms=3000,
    memory_kb=64 * 1024,
    address_space_bytes=160 * MB,
    output_bytes=64 * 1024,
    extra_tasks=16,
)


def run_status(**overrides: object) -> Status:
    fields: dict[str, object] = {
        "limits": LIMITS,
        "timed_out": False,
        "cpu_time_ms": 10,
        "memory_kb": 9000,
        "stdout_size": 3,
        "exit_code": 0,
        "term_signal": None,
        "stderr": b"",
    }
    fields.update(overrides)
    return classify(**fields)  # type: ignore[arg-type]


@pytest.mark.parametrize(
    ("overrides", "expected"),
    [
        pytest.param({}, Status.OK, id="clean-exit"),
        pytest.param({"timed_out": True, "term_signal": signal.SIGKILL}, Status.TIME_LIMIT, id="wall-timeout"),
        pytest.param({"cpu_time_ms": 1001}, Status.TIME_LIMIT, id="cpu-over-limit"),
        pytest.param({"cpu_time_ms": 1000}, Status.OK, id="cpu-at-limit"),
        pytest.param({"term_signal": signal.SIGXCPU, "exit_code": None}, Status.TIME_LIMIT, id="sigxcpu"),
        pytest.param({"term_signal": signal.SIGXFSZ, "exit_code": None}, Status.OUTPUT_LIMIT, id="sigxfsz"),
        pytest.param({"stdout_size": 64 * 1024, "exit_code": 1}, Status.OUTPUT_LIMIT, id="output-full-efbig"),
        pytest.param({"memory_kb": 64 * 1024 + 1}, Status.MEMORY_LIMIT, id="rss-over-limit"),
        pytest.param({"exit_code": 1, "stderr": b"...\nMemoryError\n"}, Status.MEMORY_LIMIT, id="python-memoryerror"),
        pytest.param({"exit_code": 0, "stderr": b"MemoryError"}, Status.OK, id="memoryerror-text-but-success"),
        pytest.param({"exit_code": 3}, Status.RUNTIME_ERROR, id="nonzero-exit"),
        pytest.param({"exit_code": None, "term_signal": signal.SIGSEGV}, Status.RUNTIME_ERROR, id="segfault"),
        pytest.param(
            {"timed_out": True, "memory_kb": 10**6, "exit_code": 1}, Status.TIME_LIMIT, id="timeout-wins-over-memory"
        ),
    ],
)
def test_classify(overrides: dict[str, object], expected: Status) -> None:
    assert run_status(**overrides) == expected


@pytest.mark.parametrize(("ms", "seconds"), [(100, 2), (1000, 2), (1001, 3), (2500, 4)])
def test_cpu_rlimit_is_above_verdict_limit(ms: int, seconds: int) -> None:
    assert cpu_limit_seconds(ms) == seconds


def run_python(tmp_path: Path, code: str, stdin: str = "", limits: ExecLimits = LIMITS) -> ExecResult:
    work = tmp_path / "work"
    io = tmp_path / "io"
    work.mkdir()
    io.mkdir()
    (work / "main.py").write_text(code, encoding="utf-8")
    (io / "stdin").write_text(stdin, encoding="utf-8")
    return execute(
        (sys.executable, "-I", "-S", "main.py"),
        cwd=str(work),
        stdin_path=str(io / "stdin"),
        io_dir=str(io),
        limits=limits,
        env={"PATH": "/usr/bin:/bin", "HOME": str(work), "LANG": "C.UTF-8"},
    )


def test_execute_reads_stdin_and_captures_stdout(tmp_path: Path) -> None:
    result = run_python(tmp_path, "a, b = map(int, input().split())\nprint(a + b)\n", stdin="2 40\n")

    assert result.status == Status.OK
    assert result.stdout == b"42\n"
    assert result.exit_code == 0


def test_execute_busy_loop_is_time_limit(tmp_path: Path) -> None:
    result = run_python(tmp_path, "while True:\n    pass\n")

    assert result.status == Status.TIME_LIMIT


def test_execute_sleep_is_stopped_by_wall_clock(tmp_path: Path) -> None:
    result = run_python(tmp_path, "import time\ntime.sleep(60)\n")

    assert result.status == Status.TIME_LIMIT
    assert result.wall_time_ms < 5000


def test_execute_memory_bomb_is_memory_limit(tmp_path: Path) -> None:
    result = run_python(tmp_path, "x = bytearray(512 * 1024 * 1024)\n")

    assert result.status == Status.MEMORY_LIMIT


def test_execute_output_flood_is_output_limit(tmp_path: Path) -> None:
    result = run_python(tmp_path, "import sys\nwhile True:\n    sys.stdout.write('x' * 4096)\n")

    assert result.status == Status.OUTPUT_LIMIT
    assert len(result.stdout) <= LIMITS.output_bytes


def test_execute_reports_exit_code(tmp_path: Path) -> None:
    result = run_python(tmp_path, "raise SystemExit(3)\n")

    assert result.status == Status.RUNTIME_ERROR
    assert result.exit_code == 3
