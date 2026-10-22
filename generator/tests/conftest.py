from dataclasses import replace

import pytest
from fastapi.testclient import TestClient

from app import main


class FakeSink:
    """In-memory sink that records every event it receives."""

    def __init__(self, fail: bool = False):
        self.fail = fail
        self.events: list[dict] = []
        self.started = False
        self.stopped = False

    async def start(self) -> None:
        self.started = True

    async def send(self, events: list[dict]) -> None:
        if self.fail:
            raise RuntimeError("sink unavailable")
        self.events.extend(events)

    async def stop(self) -> None:
        self.stopped = True


@pytest.fixture
def fake_sink():
    return FakeSink()


@pytest.fixture
def make_client(monkeypatch):
    """Builds a TestClient wired to the given sink, with the background stream disabled by default."""

    def _make(sink, **overrides):
        overrides.setdefault("events_per_second", 0)
        monkeypatch.setattr(main, "settings", replace(main.settings, **overrides))
        monkeypatch.setattr(main, "build_sink", lambda: sink)
        return TestClient(main.app)

    return _make


@pytest.fixture
def client(make_client, fake_sink):
    with make_client(fake_sink) as c:
        yield c
