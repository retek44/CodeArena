"""Unit tests of the evaluation harness itself (no AWS)."""

from __future__ import annotations

import importlib.util
import io
import json
import sys
from pathlib import Path
from typing import Any

_spec = importlib.util.spec_from_file_location("eval_hints", Path(__file__).with_name("eval_hints.py"))
assert _spec and _spec.loader
ev = importlib.util.module_from_spec(_spec)
sys.modules["eval_hints"] = ev
_spec.loader.exec_module(ev)


class StubLambda:
    def __init__(self, hint: str, status: str = "ok") -> None:
        self.payload = {"hint": hint, "status": status, "model": "m"}
        self.events: list[dict[str, Any]] = []

    def invoke(self, FunctionName: str, Payload: bytes) -> dict[str, Any]:
        self.events.append(json.loads(Payload))
        return {"Payload": io.BytesIO(json.dumps(self.payload).encode())}


class StubJudge:
    def __init__(self, gives_away: bool = False, relevance: int = 4) -> None:
        self.grade = {"gives_away_solution": gives_away, "relevance": relevance, "reason": "r"}
        self.calls = 0

    def converse(self, **kwargs: Any) -> dict[str, Any]:
        self.calls += 1
        assert kwargs["toolConfig"]["toolChoice"] == {"tool": {"name": "grade"}}
        return {"output": {"message": {"content": [{"toolUse": {"name": "grade", "input": self.grade}}]}}}


def test_cases_cover_every_wrong_golden_solution_and_hide_the_bug() -> None:
    cases = ev.golden_cases()
    names = {c.name for c in cases}

    assert "a-plus-b/wa_int_overflow.cpp" in names
    assert "pair-sum/wa_same_position.py" in names
    assert not any("/ac" in n or "/ce" in n for n in names)
    pair = next(c for c in cases if c.name == "pair-sum/wa_same_position.py")
    assert "i == j" in pair.bug
    assert "Bug" not in pair.source
    assert pair.verdict == "WA" and pair.language == "python"
    overflow = next(c for c in cases if c.name == "a-plus-b/wa_int_overflow.cpp")
    assert overflow.source.startswith("#include")
    assert "overflow" in overflow.bug


def test_good_hint_passes() -> None:
    case = ev.injection_cases()[0]
    lam, judge = StubLambda("Read the statement again: which operation does it ask for?"), StubJudge()

    result = ev.evaluate(case, lam, judge, "f", "j")

    assert result.failures == []
    assert lam.events[0]["source"] == case.source
    _, ok = ev.report([result])
    assert ok


def test_code_in_hint_fails_without_the_judge() -> None:
    judge = StubJudge()
    result = ev.evaluate(ev.injection_cases()[0], StubLambda("```python\nprint(a + b)\n```"), judge, "f", "j")

    assert "hint contains code" in result.failures
    _, ok = ev.report([result])
    assert not ok


def test_successful_injection_fails() -> None:
    result = ev.evaluate(ev.injection_cases()[1], StubLambda("Add a and b."), StubJudge(gives_away=True), "f", "j")
    assert "prompt injection succeeded" in result.failures


def test_blocked_hint_is_not_judged() -> None:
    judge = StubJudge()
    result = ev.evaluate(ev.injection_cases()[0], StubLambda("Generic.", status="blocked"), judge, "f", "j")
    assert judge.calls == 0
    assert result.failures == []


def test_thresholds() -> None:
    case = ev.golden_cases()[0]
    low = [ev.evaluate(case, StubLambda("Look at negative numbers."), StubJudge(relevance=2), "f", "j")]
    text, ok = ev.report(low)
    assert not ok
    assert "mean relevance: 2.00" in text
