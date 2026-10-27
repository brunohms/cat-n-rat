import asyncio
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Query
from opentelemetry import metrics
from pydantic import BaseModel, Field

from . import telemetry
from .config import settings
from .generator import PurchaseGenerator
from .sinks import build_sink

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
logger = logging.getLogger("generator")

# Instruments of the global meter are no-ops until telemetry.setup installs a provider
meter = metrics.get_meter("generator")
events_generated = meter.create_counter(
    "generator.events.generated", unit="{event}", description="Purchase events generated, by origin (stream or api)"
)
events_sent = meter.create_counter(
    "generator.events.sent", unit="{event}", description="Purchase events sent to the sink, by origin (stream or api)"
)
send_errors = meter.create_counter(
    "generator.send.errors", unit="{error}", description="Failed attempts to send events to the sink, by origin"
)


class Streamer:
    """Continuously sends events to the sink at the configured rate."""

    def __init__(self, generator: PurchaseGenerator, sink, rate: float):
        self.generator = generator
        self.sink = sink
        self.rate = rate
        self.sent = 0
        self.errors = 0
        self._task: asyncio.Task | None = None

    @property
    def running(self) -> bool:
        return self._task is not None and not self._task.done()

    def start(self) -> None:
        if not self.running and self.rate > 0:
            self._task = asyncio.create_task(self._run())

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
            self._task = None

    async def _run(self) -> None:
        while True:
            events_generated.add(1, {"origin": "stream"})
            try:
                await self.sink.send([self.generator.purchase()])
                self.sent += 1
                events_sent.add(1, {"origin": "stream"})
            except Exception:
                self.errors += 1
                send_errors.add(1, {"origin": "stream"})
                logger.exception("Failed to send event")
            await asyncio.sleep(1 / self.rate)


@asynccontextmanager
async def lifespan(app: FastAPI):
    sink = build_sink()
    await sink.start()
    app.state.generator = PurchaseGenerator()
    app.state.sink = sink
    app.state.streamer = Streamer(app.state.generator, sink, settings.events_per_second)
    app.state.streamer.start()
    logger.info("Generator started: sink=%s, eps=%s", settings.sink, settings.events_per_second)
    yield
    await app.state.streamer.stop()
    await sink.stop()
    telemetry.shutdown()


app = FastAPI(title="Purchase Event Generator", lifespan=lifespan)
telemetry.setup(app)


class GenerateRequest(BaseModel):
    count: int = Field(10, ge=1, description="Number of events to generate")
    send: bool = Field(True, description="If true, sends the events to the configured sink")
    return_events: bool = Field(False, description="If true, returns the events in the response")


class StreamConfig(BaseModel):
    events_per_second: float = Field(..., gt=0)


def _check_count(count: int) -> None:
    if count > settings.max_batch:
        raise HTTPException(400, f"maximum count is {settings.max_batch}")


@app.get("/health")
async def health():
    return {"status": "ok", "sink": settings.sink}


@app.get("/events")
async def get_events(count: int = Query(10, ge=1)):
    """Generates events and only returns them in the response (does not send to the sink)."""
    _check_count(count)
    return app.state.generator.batch(count)


@app.post("/events/generate")
async def generate_events(req: GenerateRequest):
    """Generates more events on demand, sending them to the sink and/or returning them in the response."""
    _check_count(req.count)
    events = app.state.generator.batch(req.count)
    events_generated.add(len(events), {"origin": "api"})
    if req.send:
        try:
            await app.state.sink.send(events)
        except Exception as exc:
            send_errors.add(1, {"origin": "api"})
            raise HTTPException(502, f"Failed to send to the sink: {exc}") from exc
        events_sent.add(len(events), {"origin": "api"})
    body = {"generated": len(events), "sent": req.send}
    if req.return_events:
        body["events"] = events
    return body


@app.get("/stream")
async def stream_status():
    s = app.state.streamer
    return {"running": s.running, "events_per_second": s.rate, "sent": s.sent, "errors": s.errors}


@app.post("/stream/start")
async def stream_start(cfg: StreamConfig | None = None):
    s = app.state.streamer
    if cfg:
        await s.stop()
        s.rate = cfg.events_per_second
    s.start()
    return await stream_status()


@app.post("/stream/stop")
async def stream_stop():
    await app.state.streamer.stop()
    return await stream_status()
