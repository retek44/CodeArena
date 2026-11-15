from __future__ import annotations

import time
import urllib.error

import pytest
import test_sandbox_security as suite


def pytest_configure(config: pytest.Config) -> None:
    # The suite also runs outside the repository (in the test container of run.sh).
    config.addinivalue_line("markers", "sandbox: needs the runner containers (SANDBOX_RUNNER_URL)")


@pytest.fixture(scope="session", autouse=True)
def runners_ready() -> None:
    """Waits until the Runtime Interface Emulator in each runner container answers."""
    for language, url in suite.URLS.items():
        if not url:
            continue
        source = (suite.PROGRAMS / f"probe.{suite.SUFFIXES[language]}").read_text(encoding="utf-8")
        deadline = time.monotonic() + 60
        while True:
            try:
                suite.invoke(language, source, ["1 2\n"])
                break
            except (urllib.error.URLError, ConnectionError, TimeoutError):
                if time.monotonic() > deadline:
                    raise
                time.sleep(1)
