"""Runner protocol v1. Mirrors runners/protocol/*.schema.json.

The runner has no third-party dependencies, so validation is written by hand.
The schema files are the contract; tests check that both sides agree.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from enum import StrEnum
from typing import Any

PROTOCOL_VERSION = 1

_ID_PATTERN = re.compile(r"^[A-Za-z0-9._-]{1,64}$")
MAX_SOURCE_CHARS = 65536
MAX_TEST_CASES = 100
# Lambda's synchronous response limit is 6 MB; keep headroom for JSON overhead.
MAX_TOTAL_OUTPUT_KB = 5 * 1024


class InvalidRequestError(ValueError):
    """The event does not match the runner protocol."""


class Status(StrEnum):
    OK = "ok"
    TIME_LIMIT = "time_limit"
    MEMORY_LIMIT = "memory_limit"
    OUTPUT_LIMIT = "output_limit"
    RUNTIME_ERROR = "runtime_error"
    SKIPPED = "skipped"


class CompileStatus(StrEnum):
    OK = "ok"
    ERROR = "error"
    TIME_LIMIT = "time_limit"


@dataclass(frozen=True)
class Limits:
    time_ms: int
    memory_mb: int
    output_kb: int = 1024
    compile_time_ms: int = 10000


@dataclass(frozen=True)
class TestCase:
    __test__ = False  # not a pytest test class

    id: str
    input: str


@dataclass(frozen=True)
class Request:
    submission_id: str
    language: str
    source: str
    limits: Limits
    test_cases: tuple[TestCase, ...]
    stop_on_failure: bool = True


@dataclass(frozen=True)
class CaseResult:
    id: str
    status: Status
    cpu_time_ms: int = 0
    wall_time_ms: int = 0
    memory_kb: int = 0
    exit_code: int | None = None
    signal: int | None = None
    stdout: str = ""
    stderr: str = ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "status": str(self.status),
            "cpu_time_ms": self.cpu_time_ms,
            "wall_time_ms": self.wall_time_ms,
            "memory_kb": self.memory_kb,
            "exit_code": self.exit_code,
            "signal": self.signal,
            "stdout": self.stdout,
            "stderr": self.stderr,
        }


@dataclass(frozen=True)
class CompileResult:
    status: CompileStatus
    output: str = ""


@dataclass
class Response:
    submission_id: str
    compile: CompileResult
    results: list[CaseResult] = field(default_factory=list)
    truncated: bool = False

    def to_dict(self) -> dict[str, Any]:
        return {
            "protocol_version": PROTOCOL_VERSION,
            "submission_id": self.submission_id,
            "compile": {"status": str(self.compile.status), "output": self.compile.output},
            "results": [r.to_dict() for r in self.results],
            "truncated": self.truncated,
        }


def parse_request(event: Any, supported_languages: frozenset[str]) -> Request:
    """Validate a Lambda event and turn it into a Request."""
    obj = _require_object(event, "event")
    _reject_unknown(
        obj,
        {"protocol_version", "submission_id", "language", "source", "limits", "test_cases", "stop_on_failure"},
        "event",
    )

    if obj.get("protocol_version") != PROTOCOL_VERSION:
        raise InvalidRequestError(f"protocol_version must be {PROTOCOL_VERSION}")

    submission_id = _require_id(obj.get("submission_id"), "submission_id")

    language = obj.get("language")
    if language not in supported_languages:
        raise InvalidRequestError(f"language must be one of {sorted(supported_languages)}")

    source = obj.get("source")
    if not isinstance(source, str) or not 1 <= len(source) <= MAX_SOURCE_CHARS:
        raise InvalidRequestError(f"source must be a string of 1..{MAX_SOURCE_CHARS} characters")

    limits = _parse_limits(obj.get("limits"))

    raw_cases = obj.get("test_cases")
    if not isinstance(raw_cases, list) or not 1 <= len(raw_cases) <= MAX_TEST_CASES:
        raise InvalidRequestError(f"test_cases must be a list of 1..{MAX_TEST_CASES} items")
    cases = tuple(_parse_case(c, i) for i, c in enumerate(raw_cases))
    if len({c.id for c in cases}) != len(cases):
        raise InvalidRequestError("test case ids must be unique")
    if len(cases) * limits.output_kb > MAX_TOTAL_OUTPUT_KB:
        raise InvalidRequestError(f"test_cases x output_kb must not exceed {MAX_TOTAL_OUTPUT_KB} KB")

    stop_on_failure = obj.get("stop_on_failure", True)
    if not isinstance(stop_on_failure, bool):
        raise InvalidRequestError("stop_on_failure must be a boolean")

    return Request(
        submission_id=submission_id,
        language=language,
        source=source,
        limits=limits,
        test_cases=cases,
        stop_on_failure=stop_on_failure,
    )


def _parse_limits(value: Any) -> Limits:
    obj = _require_object(value, "limits")
    _reject_unknown(obj, {"time_ms", "memory_mb", "output_kb", "compile_time_ms"}, "limits")
    return Limits(
        time_ms=_require_int(obj.get("time_ms"), "limits.time_ms", 100, 10000),
        memory_mb=_require_int(obj.get("memory_mb"), "limits.memory_mb", 16, 512),
        output_kb=_require_int(obj.get("output_kb", 1024), "limits.output_kb", 1, 4096),
        compile_time_ms=_require_int(obj.get("compile_time_ms", 10000), "limits.compile_time_ms", 1000, 30000),
    )


def _parse_case(value: Any, index: int) -> TestCase:
    name = f"test_cases[{index}]"
    obj = _require_object(value, name)
    _reject_unknown(obj, {"id", "input"}, name)
    case_input = obj.get("input")
    if not isinstance(case_input, str):
        raise InvalidRequestError(f"{name}.input must be a string")
    return TestCase(id=_require_id(obj.get("id"), f"{name}.id"), input=case_input)


def _require_object(value: Any, name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise InvalidRequestError(f"{name} must be an object")
    return value


def _reject_unknown(obj: dict[str, Any], allowed: set[str], name: str) -> None:
    unknown = set(obj) - allowed
    if unknown:
        raise InvalidRequestError(f"{name} has unknown fields: {sorted(unknown)}")


def _require_id(value: Any, name: str) -> str:
    if not isinstance(value, str) or not _ID_PATTERN.match(value):
        raise InvalidRequestError(f"{name} must match {_ID_PATTERN.pattern}")
    return value


def _require_int(value: Any, name: str, low: int, high: int) -> int:
    # bool is a subclass of int; reject it explicitly.
    if isinstance(value, bool) or not isinstance(value, int) or not low <= value <= high:
        raise InvalidRequestError(f"{name} must be an integer in {low}..{high}")
    return value
