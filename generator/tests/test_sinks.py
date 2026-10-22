import asyncio
import json
import sys
import types
from dataclasses import replace

import httpx
import pytest

from app import sinks
from app.sinks import HttpSink, KafkaSink, StdoutSink, build_sink

EVENTS = [{"order_id": "ORD-00000001", "total": 10.0}, {"order_id": "ORD-00000002", "total": 20.0}]


def test_stdout_sink_writes_one_json_line_per_event(capsys):
    async def run():
        sink = StdoutSink()
        await sink.start()
        await sink.send(EVENTS)
        await sink.stop()

    asyncio.run(run())

    lines = capsys.readouterr().out.splitlines()
    assert [json.loads(line) for line in lines] == EVENTS


def test_stdout_sink_keeps_non_ascii_characters(capsys):
    asyncio.run(StdoutSink().send([{"name": "Sachê"}]))
    assert "Sachê" in capsys.readouterr().out


def test_http_sink_posts_events_as_json():
    requests = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        return httpx.Response(200)

    async def run():
        sink = HttpSink("http://target/events")
        sink.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        await sink.send(EVENTS)
        await sink.stop()

    asyncio.run(run())

    assert len(requests) == 1
    assert requests[0].method == "POST"
    assert str(requests[0].url) == "http://target/events"
    assert json.loads(requests[0].content) == EVENTS


def test_http_sink_raises_on_error_status():
    async def run():
        sink = HttpSink("http://target/events")
        sink.client = httpx.AsyncClient(transport=httpx.MockTransport(lambda r: httpx.Response(500)))
        try:
            await sink.send(EVENTS)
        finally:
            await sink.stop()

    with pytest.raises(httpx.HTTPStatusError):
        asyncio.run(run())


def test_http_sink_stop_without_start_is_safe():
    asyncio.run(HttpSink("http://target/events").stop())


class FakeProducer:
    instances: list["FakeProducer"] = []

    def __init__(self, bootstrap_servers, value_serializer, key_serializer):
        self.bootstrap_servers = bootstrap_servers
        self.value_serializer = value_serializer
        self.key_serializer = key_serializer
        self.sent = []
        self.started = self.stopped = self.flushed = False
        FakeProducer.instances.append(self)

    async def start(self):
        self.started = True

    async def send(self, topic, value, key):
        self.sent.append((topic, self.value_serializer(value), self.key_serializer(key)))

    async def flush(self):
        self.flushed = True

    async def stop(self):
        self.stopped = True


def test_kafka_sink_sends_each_event_keyed_by_order_id(monkeypatch):
    monkeypatch.setitem(sys.modules, "aiokafka", types.SimpleNamespace(AIOKafkaProducer=FakeProducer))
    FakeProducer.instances.clear()

    async def run():
        sink = KafkaSink("broker:9092", "purchases")
        await sink.start()
        await sink.send(EVENTS)
        await sink.stop()

    asyncio.run(run())

    producer = FakeProducer.instances[0]
    assert producer.bootstrap_servers == "broker:9092"
    assert producer.started and producer.flushed and producer.stopped
    assert producer.sent == [
        ("purchases", json.dumps(EVENTS[0]).encode(), b"ORD-00000001"),
        ("purchases", json.dumps(EVENTS[1]).encode(), b"ORD-00000002"),
    ]


def test_kafka_sink_stop_without_start_is_safe():
    asyncio.run(KafkaSink("broker:9092", "purchases").stop())


@pytest.mark.parametrize(
    ("sink_name", "expected_type"),
    [("stdout", StdoutSink), ("http", HttpSink), ("kafka", KafkaSink), ("unknown", StdoutSink)],
)
def test_build_sink_selects_sink_from_settings(monkeypatch, sink_name, expected_type):
    monkeypatch.setattr(sinks, "settings", replace(sinks.settings, sink=sink_name))
    assert isinstance(build_sink(), expected_type)


def test_build_sink_uses_configured_http_url(monkeypatch):
    monkeypatch.setattr(sinks, "settings", replace(sinks.settings, sink="http", http_target_url="http://x/y"))
    assert build_sink().url == "http://x/y"


def test_build_sink_uses_configured_kafka_settings(monkeypatch):
    monkeypatch.setattr(
        sinks, "settings",
        replace(sinks.settings, sink="kafka", kafka_bootstrap_servers="k:1", kafka_topic="orders"),
    )
    sink = build_sink()
    assert (sink.bootstrap_servers, sink.topic) == ("k:1", "orders")
