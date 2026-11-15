"""Sandbox hardening suite (PLAN.md, PR pipeline step 6; ADR-0003).

Runs probe programs through the real runner images under the Lambda Runtime Interface
Emulator, the way the worker invokes them on AWS, and checks two things per scenario:

  1. the expected outcome: the limit that must stop the program, or the output of a
     program that reports what the sandbox let it do (network, file system, environment);
  2. a clean environment afterwards: a probe run in the same execution environment finds
     no process or file left by the scenario, and still judges a normal program.

The runner containers must run like Lambda (non-root, read-only root file system,
writable /tmp only) on a network without a route out; `make sandbox-suite` and the CI job
start them that way. A language without its runner URL (SANDBOX_RUNNER_URL for Python,
SANDBOX_RUNNER_URL_<LANGUAGE> for the others) is skipped.
"""

from __future__ import annotations

import json
import os
import urllib.request
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pytest

pytestmark = pytest.mark.sandbox

PROGRAMS = Path(__file__).parent / "programs"
EXTENSIONS = {"py": "python", "cpp": "cpp", "java": "java", "js": "javascript", "go": "go"}
SUFFIXES = {language: extension for extension, language in EXTENSIONS.items()}
URLS = {
    language: os.environ.get("SANDBOX_RUNNER_URL" + ("" if language == "python" else f"_{language.upper()}"), "")
    for language in EXTENSIONS.values()
}
TIME_MS = 1000
MEMORY_MB = 64
# The runner's wall-clock deadline is 2 x the time limit + 1 s (handler._run_limits).
WALL_LIMIT_MS = TIME_MS * 2 + 1000


@dataclass(frozen=True)
class Scenario:
    program: str
    status: str
    stdout: str | None = None
    inputs: tuple[str, ...] = ("",)

    @property
    def language(self) -> str:
        return EXTENSIONS[self.program.rsplit(".", 1)[1]]


SCENARIOS = [
    # Limits
    Scenario("busy_loop.py", "time_limit"),
    Scenario("sleep.py", "time_limit"),
    Scenario("memory.py", "memory_limit"),
    Scenario("output_flood.py", "output_limit"),
    Scenario("process_limit.py", "ok", "limited\n"),
    # Isolation
    Scenario("network.py", "ok", "tcp: blocked\ntcp: blocked\ndns: blocked\n"),
    Scenario(
        "filesystem.py",
        "ok",
        "/ denied\n/etc denied\n/var/task denied\n/opt denied\ncwd written\ntmp written\n",
    ),
    Scenario("environment.py", "ok", "HOME LANG PATH TMPDIR\n"),
    Scenario("credentials.py", "ok", "credentials: none\n"),
    # Leftovers
    Scenario("leftover_process.py", "ok", "started\n"),
    # Compiled programs run from /tmp in the C++ image.
    Scenario("busy_loop.cpp", "time_limit"),
    Scenario("memory.cpp", "memory_limit"),
    Scenario("process_limit.cpp", "ok", "limited\n"),
    Scenario("network.cpp", "ok", "tcp: blocked\ntcp: blocked\n"),
    # Managed runtimes: heap limits from the problem's memory limit, helper threads.
    *(
        scenario
        for extension in ("java", "js", "go")
        for scenario in (
            Scenario(f"busy_loop.{extension}", "time_limit"),
            Scenario(f"memory.{extension}", "memory_limit"),
            Scenario(f"network.{extension}", "ok", "tcp: blocked\ntcp: blocked\n"),
        )
    ),
]

CLEAN = {
    "python": "processes: 0\nfiles: 0\n3\n",
    "cpp": "processes: 0\n3\n",
    "java": "processes: 0\n3\n",
    "javascript": "processes: 0\n3\n",
    "go": "processes: 0\n3\n",
}


def invoke(language: str, source: str, inputs: list[str]) -> dict[str, Any]:
    url = URLS[language]
    if not url:
        pytest.skip(f"no runner for {language}")
    request = {
        "protocol_version": 1,
        "submission_id": f"sandbox-{uuid.uuid4().hex[:12]}",
        "language": language,
        "source": source,
        "limits": {"time_ms": TIME_MS, "memory_mb": MEMORY_MB},
        "test_cases": [{"id": str(i + 1), "input": text} for i, text in enumerate(inputs)],
        "stop_on_failure": False,
    }
    body = json.dumps(request).encode()
    http = urllib.request.Request(url, data=body, headers={"Content-Type": "application/json"})  # noqa: S310
    with urllib.request.urlopen(http, timeout=120) as response:  # noqa: S310 - local runner URL
        result: dict[str, Any] = json.loads(response.read())
    assert "errorType" not in result, f"runner invocation failed: {result}"
    return result


def run(language: str, program: str, inputs: list[str]) -> dict[str, Any]:
    return invoke(language, (PROGRAMS / program).read_text(encoding="utf-8"), inputs)


def assert_clean(language: str, after: str) -> None:
    """The next run in the same environment finds nothing left and is judged normally."""
    probe = run(language, f"probe.{SUFFIXES[language]}", ["1 2\n"])
    assert probe["compile"]["status"] == "ok", probe["compile"]
    (case,) = probe["results"]
    assert case["status"] == "ok", f"probe after {after}: {case}"
    assert case["stdout"] == CLEAN[language], f"leftovers after {after}"


@pytest.mark.parametrize("scenario", SCENARIOS, ids=lambda s: s.program)
def test_scenario_is_contained(scenario: Scenario) -> None:
    response = run(scenario.language, scenario.program, list(scenario.inputs))

    assert response["compile"]["status"] == "ok", response["compile"]
    (case,) = response["results"]
    assert case["status"] == scenario.status, f"{case['status']}: {case.get('stderr', '')!r}"
    if scenario.stdout is not None:
        assert case["stdout"] == scenario.stdout
    if scenario.status == "time_limit":
        assert case["wall_time_ms"] <= WALL_LIMIT_MS + 1000, "the deadline must stop the program in time"
    assert_clean(scenario.language, scenario.program)


def test_test_cases_do_not_see_each_other() -> None:
    token = uuid.uuid4().hex
    response = run("python", "case_isolation.py", [f"write {token}\n", f"search {token}\n"])

    first, second = response["results"]
    assert (first["status"], first["stdout"]) == ("ok", "written\n")
    assert (second["status"], second["stdout"]) == ("ok", "found: 0\n")
    assert_clean("python", "case_isolation.py")


def test_submissions_do_not_see_each_other() -> None:
    token = uuid.uuid4().hex
    run("python", "case_isolation.py", [f"write {token}\n"])
    response = run("python", "case_isolation.py", [f"search {token}\n"])

    (case,) = response["results"]
    assert (case["status"], case["stdout"]) == ("ok", "found: 0\n")
