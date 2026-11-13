from __future__ import annotations

from typing import Any

import pytest

from codearena_hint import handler


class StubBedrock:
    def __init__(self, text: str = "Check what happens when n is 0.", stop_reason: str = "end_turn") -> None:
        self.text = text
        self.stop_reason = stop_reason
        self.requests: list[dict[str, Any]] = []

    def converse(self, **kwargs: Any) -> dict[str, Any]:
        self.requests.append(kwargs)
        return {
            "output": {"message": {"role": "assistant", "content": [{"text": self.text}]}},
            "stopReason": self.stop_reason,
            "usage": {"inputTokens": 100, "outputTokens": 20},
        }


def event(**overrides: Any) -> dict[str, Any]:
    base: dict[str, Any] = {
        "problem_title": "A + B",
        "statement": "Print the sum of two integers.",
        "language": "python",
        "source": "a, b = map(int, input().split())\nprint(a - b)\n",
        "verdict": "WA",
        "failed_case": {"ordinal": 1, "is_sample": True, "input": "1 2", "expected": "3"},
    }
    base.update(overrides)
    return base


def user_texts(request: dict[str, Any]) -> tuple[str, str]:
    content = request["messages"][0]["content"]
    plain = "\n".join(block["text"] for block in content if "text" in block)
    guarded = "\n".join(block["guardContent"]["text"]["text"] for block in content if "guardContent" in block)
    return plain, guarded


def test_returns_the_model_hint_with_guardrail() -> None:
    stub = StubBedrock()
    got = handler.make_hint(event(), stub, "eu.anthropic.claude-haiku-5-5", ("gr-1", "3"))

    assert got == {"hint": "Check what happens when n is 0.", "status": "ok", "model": "eu.anthropic.claude-haiku-5-5"}
    request = stub.requests[0]
    assert request["guardrailConfig"]["guardrailIdentifier"] == "gr-1"
    assert request["guardrailConfig"]["guardrailVersion"] == "3"
    assert request["inferenceConfig"]["maxTokens"] <= 300
    assert "No Markdown" in request["system"][0]["text"]


def test_only_the_source_is_guarded_content() -> None:
    stub = StubBedrock()
    handler.make_hint(event(), stub, "m", None)

    plain, guarded = user_texts(stub.requests[0])
    assert "print(a - b)" in guarded
    assert "print(a - b)" not in plain
    assert "Expected output:\n3" in plain
    assert "guardrailConfig" not in stub.requests[0]


def test_hidden_case_data_is_not_sent() -> None:
    stub = StubBedrock()
    hidden = {"ordinal": 4, "is_sample": False, "input": "secret-in", "expected": "secret-out"}
    handler.make_hint(event(failed_case=hidden), stub, "m", None)

    plain, guarded = user_texts(stub.requests[0])
    assert "hidden test #4" in plain
    assert "secret-in" not in plain + guarded
    assert "secret-out" not in plain + guarded


def test_long_inputs_are_clipped() -> None:
    stub = StubBedrock()
    handler.make_hint(event(source="x" * 50_000, statement="s" * 50_000), stub, "m", None)

    plain, guarded = user_texts(stub.requests[0])
    assert len(guarded) < handler.MAX_SOURCE_CHARS + 20
    assert len(plain) < handler.MAX_STATEMENT_CHARS + 1_000


@pytest.mark.parametrize(
    "answer",
    [
        "Try this:\n```python\nprint(a + b)\n```",
        "Use:\nfor i in range(n):\n    total += a[i]",
        "#include <bits/stdc++.h>\nint main() { }",
        "long long s = a + b;\ncout << s;",
        "Replace the loop with std::accumulate.",
    ],
)
def test_code_answers_are_replaced(answer: str) -> None:
    got = handler.make_hint(event(verdict="TLE"), StubBedrock(text=answer), "m", None)

    assert got["status"] == "blocked"
    assert got["hint"] == handler.FALLBACK_HINTS["TLE"]


@pytest.mark.parametrize(
    "answer",
    [
        "Your subtraction does not match what the statement asks for; re-read the operation.",
        "Think about the largest values: can the sum still fit in the type you use?",
        "If the list is empty, what does your loop return?",
    ],
)
def test_prose_answers_pass(answer: str) -> None:
    got = handler.make_hint(event(), StubBedrock(text=answer), "m", None)
    assert got["status"] == "ok"
    assert got["hint"] == answer


def test_guardrail_intervention_gives_the_fallback() -> None:
    stub = StubBedrock(text="Sorry, I can't help with that.", stop_reason="guardrail_intervened")
    got = handler.make_hint(event(verdict="RE"), stub, "m", ("gr", "1"))
    assert got == {"hint": handler.FALLBACK_HINTS["RE"], "status": "blocked", "model": "m"}


def test_empty_answer_and_unknown_verdict() -> None:
    got = handler.make_hint(event(verdict="IE"), StubBedrock(text="  "), "m", None)
    assert got == {"hint": handler.GENERIC_HINT, "status": "blocked", "model": "m"}


def test_handler_reads_the_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    stub = StubBedrock()
    monkeypatch.setattr(handler, "_client", stub)
    monkeypatch.setenv("MODEL_ID", "model-from-env")
    monkeypatch.setenv("GUARDRAIL_ID", "gr-env")
    monkeypatch.setenv("GUARDRAIL_VERSION", "7")

    got = handler.handler(event(), None)

    assert got["model"] == "model-from-env"
    assert stub.requests[0]["guardrailConfig"]["guardrailVersion"] == "7"
