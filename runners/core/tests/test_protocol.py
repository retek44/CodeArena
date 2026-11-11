import copy
import json
from pathlib import Path
from typing import Any

import jsonschema
import pytest

from codearena_runner.protocol import (
    CaseResult,
    CompileResult,
    CompileStatus,
    InvalidRequestError,
    Response,
    Status,
    parse_request,
)

SCHEMA_DIR = Path(__file__).resolve().parents[2] / "protocol"
REQUEST_SCHEMA = json.loads((SCHEMA_DIR / "request.schema.json").read_text(encoding="utf-8"))
RESPONSE_SCHEMA = json.loads((SCHEMA_DIR / "response.schema.json").read_text(encoding="utf-8"))
LANGUAGES = frozenset({"python"})

VALID: dict[str, Any] = {
    "protocol_version": 1,
    "submission_id": "sub-123",
    "language": "python",
    "source": "print(1)",
    "limits": {"time_ms": 1000, "memory_mb": 64},
    "test_cases": [{"id": "1", "input": "1 2\n"}, {"id": "2", "input": ""}],
}


def with_change(path: str, value: Any) -> dict[str, Any]:
    """Copy of VALID with one field set; nested fields use dots ("limits.time_ms", "test_cases.0.input")."""
    event = copy.deepcopy(VALID)
    *parents, last = path.split(".")
    target = event
    for key in parents:
        target = target[int(key)] if key.isdigit() else target[key]
    target[last] = value
    return event


def test_valid_request_parses_with_defaults() -> None:
    request = parse_request(VALID, LANGUAGES)

    assert request.submission_id == "sub-123"
    assert request.limits.output_kb == 1024
    assert request.limits.compile_time_ms == 10000
    assert request.stop_on_failure is True
    assert [c.id for c in request.test_cases] == ["1", "2"]


# Rejected by both the parser and the JSON Schema.
INVALID = [
    pytest.param("not an object", id="event-not-object"),
    pytest.param(with_change("protocol_version", 2), id="wrong-protocol-version"),
    pytest.param(with_change("submission_id", "a/b"), id="submission-id-bad-chars"),
    pytest.param(with_change("submission_id", "x" * 65), id="submission-id-too-long"),
    pytest.param(with_change("language", "cobol"), id="unsupported-language"),
    pytest.param(with_change("source", ""), id="empty-source"),
    pytest.param(with_change("source", "x" * 65537), id="source-too-long"),
    pytest.param(with_change("limits.time_ms", 99), id="time-below-min"),
    pytest.param(with_change("limits.time_ms", 10001), id="time-above-max"),
    pytest.param(with_change("limits.time_ms", True), id="time-is-bool"),
    pytest.param(with_change("limits.memory_mb", "64"), id="memory-is-string"),
    pytest.param(with_change("limits.extra", 1), id="limits-unknown-field"),
    pytest.param(with_change("test_cases", []), id="no-test-cases"),
    pytest.param(with_change("test_cases.0.input", 5), id="case-input-not-string"),
    pytest.param(with_change("test_cases.0.expected", "3"), id="expected-output-not-accepted"),
    pytest.param(with_change("unknown", 1), id="event-unknown-field"),
    pytest.param(with_change("stop_on_failure", "yes"), id="stop-on-failure-not-bool"),
    pytest.param(with_change("test_cases", "1"), id="test-cases-not-list"),
]

# Cross-field rules that JSON Schema cannot express; only the parser rejects them.
INVALID_AT_RUNTIME = [
    pytest.param(with_change("test_cases", [{"id": "1", "input": ""}] * 2), id="duplicate-case-ids"),
    pytest.param(
        with_change("test_cases", [{"id": str(i), "input": ""} for i in range(6)])
        | {"limits": {"time_ms": 1000, "memory_mb": 64, "output_kb": 1000}},
        id="total-output-too-large",
    ),
]


@pytest.mark.parametrize("event", INVALID + INVALID_AT_RUNTIME)
def test_invalid_request_is_rejected(event: Any) -> None:
    with pytest.raises(InvalidRequestError):
        parse_request(event, LANGUAGES)


def test_valid_example_matches_schema() -> None:
    jsonschema.validate(VALID, REQUEST_SCHEMA)


@pytest.mark.parametrize("event", INVALID)
def test_schema_and_parser_agree_on_invalid_requests(event: Any) -> None:
    """Contract check: the worker validates against the schema, the runner with the parser."""
    with pytest.raises(jsonschema.ValidationError):
        jsonschema.validate(event, REQUEST_SCHEMA)


def test_response_matches_schema() -> None:
    response = Response(
        submission_id="sub-123",
        compile=CompileResult(CompileStatus.OK),
        results=[
            CaseResult(id="1", status=Status.OK, cpu_time_ms=12, wall_time_ms=20, memory_kb=9000, exit_code=0),
            CaseResult(id="2", status=Status.TIME_LIMIT, signal=9),
            CaseResult(id="3", status=Status.SKIPPED),
        ],
    )

    jsonschema.validate(response.to_dict(), RESPONSE_SCHEMA)


def test_status_enums_match_schema() -> None:
    case_statuses = RESPONSE_SCHEMA["properties"]["results"]["items"]["properties"]["status"]["enum"]
    compile_statuses = RESPONSE_SCHEMA["properties"]["compile"]["properties"]["status"]["enum"]

    assert sorted(case_statuses) == sorted(s.value for s in Status)
    assert sorted(compile_statuses) == sorted(s.value for s in CompileStatus)
