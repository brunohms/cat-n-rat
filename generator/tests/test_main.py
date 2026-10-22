import asyncio

from app.generator import PurchaseGenerator
from app.main import Streamer

from .conftest import FakeSink


def test_health(client):
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok", "sink": "stdout"}


def test_lifespan_starts_and_stops_sink(make_client, fake_sink):
    with make_client(fake_sink):
        assert fake_sink.started
    assert fake_sink.stopped


def test_get_events_returns_events_without_sending(client, fake_sink):
    response = client.get("/events", params={"count": 5})

    assert response.status_code == 200
    assert len(response.json()) == 5
    assert fake_sink.events == []


def test_get_events_defaults_to_ten(client):
    assert len(client.get("/events").json()) == 10


def test_get_events_rejects_invalid_count(client):
    assert client.get("/events", params={"count": 0}).status_code == 422


def test_get_events_rejects_count_above_max_batch(make_client, fake_sink):
    with make_client(fake_sink, max_batch=3) as client:
        response = client.get("/events", params={"count": 4})

    assert response.status_code == 400
    assert response.json()["detail"] == "maximum count is 3"


def test_generate_sends_events_to_sink(client, fake_sink):
    response = client.post("/events/generate", json={"count": 7})

    assert response.status_code == 200
    assert response.json() == {"generated": 7, "sent": True}
    assert len(fake_sink.events) == 7


def test_generate_can_return_events_without_sending(client, fake_sink):
    response = client.post("/events/generate", json={"count": 3, "send": False, "return_events": True})

    body = response.json()
    assert body["generated"] == 3 and body["sent"] is False
    assert len(body["events"]) == 3
    assert fake_sink.events == []


def test_generate_returned_events_match_sent_events(client, fake_sink):
    body = client.post("/events/generate", json={"count": 3, "return_events": True}).json()
    assert body["events"] == fake_sink.events


def test_generate_rejects_count_above_max_batch(make_client, fake_sink):
    with make_client(fake_sink, max_batch=5) as client:
        response = client.post("/events/generate", json={"count": 6})

    assert response.status_code == 400
    assert fake_sink.events == []


def test_generate_returns_502_when_sink_fails(make_client):
    with make_client(FakeSink(fail=True)) as client:
        response = client.post("/events/generate", json={"count": 1})

    assert response.status_code == 502
    assert "sink unavailable" in response.json()["detail"]


def test_stream_is_stopped_when_rate_is_zero(client):
    assert client.get("/stream").json() == {"running": False, "events_per_second": 0, "sent": 0, "errors": 0}


def test_stream_starts_automatically_with_positive_rate(make_client, fake_sink):
    with make_client(fake_sink, events_per_second=1) as client:
        assert client.get("/stream").json()["running"] is True


def test_stream_start_with_new_rate_and_stop(client):
    started = client.post("/stream/start", json={"events_per_second": 50}).json()
    assert started["running"] is True
    assert started["events_per_second"] == 50

    stopped = client.post("/stream/stop").json()
    assert stopped["running"] is False


def test_stream_start_rejects_non_positive_rate(client):
    assert client.post("/stream/start", json={"events_per_second": 0}).status_code == 422


def test_stream_start_without_rate_keeps_disabled_rate(client):
    assert client.post("/stream/start").json()["running"] is False


def run_streamer(sink, rate: float, seconds: float) -> Streamer:
    async def run():
        streamer = Streamer(PurchaseGenerator(seed=1), sink, rate)
        streamer.start()
        await asyncio.sleep(seconds)
        await streamer.stop()
        return streamer

    return asyncio.run(run())


def test_streamer_sends_events_continuously():
    sink = FakeSink()
    streamer = run_streamer(sink, rate=200, seconds=0.1)

    assert streamer.sent > 1
    assert len(sink.events) == streamer.sent
    assert streamer.errors == 0
    assert not streamer.running


def test_streamer_counts_errors_and_keeps_running():
    streamer = run_streamer(FakeSink(fail=True), rate=200, seconds=0.1)

    assert streamer.sent == 0
    assert streamer.errors > 1


def test_streamer_does_not_start_with_zero_rate():
    async def run():
        streamer = Streamer(PurchaseGenerator(seed=1), FakeSink(), 0)
        streamer.start()
        return streamer.running

    assert asyncio.run(run()) is False


def test_streamer_start_is_idempotent():
    async def run():
        streamer = Streamer(PurchaseGenerator(seed=1), FakeSink(), 10)
        streamer.start()
        task = streamer._task
        streamer.start()
        same = streamer._task is task
        await streamer.stop()
        return same

    assert asyncio.run(run()) is True
