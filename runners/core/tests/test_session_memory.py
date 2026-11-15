import pytest

from codearena_runner.handler import session_memory_kb


def test_session_memory_is_the_lambda_memory_minus_the_runner_reserve(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", "1024")
    assert session_memory_kb() == (1024 - 160) * 1024


@pytest.mark.parametrize("value", [None, "", "lots"])
def test_no_session_memory_outside_lambda(monkeypatch: pytest.MonkeyPatch, value: str | None) -> None:
    if value is None:
        monkeypatch.delenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", raising=False)
    else:
        monkeypatch.setenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", value)
    assert session_memory_kb() is None
