"""Language selection per runner image (CODEARENA_LANGUAGES)."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from codearena_runner.languages import LANGUAGES, enabled_languages

REQUEST_SCHEMA = json.loads((Path(__file__).parents[2] / "protocol" / "request.schema.json").read_text())


def test_default_is_python() -> None:
    assert list(enabled_languages({})) == ["python"]


def test_selects_listed_languages() -> None:
    assert list(enabled_languages({"CODEARENA_LANGUAGES": " cpp , python "})) == ["cpp", "python"]


@pytest.mark.parametrize("raw", ["cobol", "python,cobol", "", " , "])
def test_rejects_unknown_or_empty(raw: str) -> None:
    with pytest.raises(ValueError, match="CODEARENA_LANGUAGES"):
        enabled_languages({"CODEARENA_LANGUAGES": raw})


def test_languages_match_schema() -> None:
    assert sorted(LANGUAGES) == sorted(REQUEST_SCHEMA["properties"]["language"]["enum"])


def test_commands_use_absolute_paths() -> None:
    # The sandbox calls execve without a PATH search.
    for language in LANGUAGES.values():
        for argv in (language.run, language.compile):
            if argv is None:
                continue
            program = argv[0]
            assert program.startswith("/") or program.startswith("./"), f"{language.name}: {program}"
