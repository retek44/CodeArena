"""Language selection per runner image (CODEARENA_LANGUAGES)."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from codearena_runner.languages import LANGUAGES, MEMORY_MB, enabled_languages

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


def test_run_command_fills_in_the_memory_limit() -> None:
    java = LANGUAGES["java"].run_command(128)
    assert "-Xmx128m" in java
    assert "--max-old-space-size=64" in LANGUAGES["javascript"].run_command(64)
    for language in LANGUAGES.values():
        assert not any(MEMORY_MB in arg for arg in language.run_command(64)), language.name


def test_compiled_languages_keep_only_what_the_run_needs() -> None:
    # The source and compiler leftovers are not written into every test case directory.
    assert LANGUAGES["cpp"].artifacts == ("main",)
    assert LANGUAGES["go"].artifacts == ("main",)
    assert LANGUAGES["java"].artifacts == ("*.class",)


def test_managed_runtimes_get_room_for_their_own_memory() -> None:
    for name in ("java", "javascript"):
        language = LANGUAGES[name]
        assert language.memory_overhead_mb > 0, name
        assert language.run_tasks > 16, name
    assert LANGUAGES["python"].memory_overhead_mb == 0
    assert LANGUAGES["cpp"].memory_overhead_mb == 0


def test_language_environment_cannot_replace_the_fixed_variables() -> None:
    for language in LANGUAGES.values():
        names = {name for name, _ in language.env}
        assert not names & {"PATH", "HOME", "TMPDIR", "LANG"}, language.name
