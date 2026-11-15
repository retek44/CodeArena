import ctypes
import errno

import pytest

from codearena_runner import linux


class FakeLibc:
    """prctl that fails with the given errno for some options."""

    def __init__(self, failures: dict[int, int]) -> None:
        self.failures = failures
        self.calls: list[tuple[int, int]] = []

    def prctl(self, option: int, value: int, *_: int) -> int:
        self.calls.append((option, value))
        if option in self.failures:
            ctypes.set_errno(self.failures[option])
            return -1
        return 0


def use_libc(monkeypatch: pytest.MonkeyPatch, libc: FakeLibc) -> None:
    monkeypatch.setattr(linux.ctypes, "CDLL", lambda *_, **__: libc)


def test_hardening_sets_both_protections(monkeypatch: pytest.MonkeyPatch) -> None:
    libc = FakeLibc({})
    use_libc(monkeypatch, libc)

    assert linux.harden_current_process() == []
    assert libc.calls == [(4, 0), (36, 1)]


def test_hardening_refused_by_seccomp_is_reported_not_raised(monkeypatch: pytest.MonkeyPatch) -> None:
    # Lambda: PR_SET_DUMPABLE fails with EPERM. The subreaper is still attempted.
    libc = FakeLibc({4: errno.EPERM})
    use_libc(monkeypatch, libc)

    assert linux.harden_current_process() == ["not_dumpable"]
    assert libc.calls == [(4, 0), (36, 1)]


def test_hardening_reports_every_refused_protection(monkeypatch: pytest.MonkeyPatch) -> None:
    use_libc(monkeypatch, FakeLibc({4: errno.EPERM, 36: errno.EPERM}))

    assert linux.harden_current_process() == ["not_dumpable", "child_subreaper"]


def test_hardening_raises_on_other_errors(monkeypatch: pytest.MonkeyPatch) -> None:
    use_libc(monkeypatch, FakeLibc({36: errno.EINVAL}))

    with pytest.raises(OSError, match=r"prctl\(36, 1\) failed"):
        linux.harden_current_process()
