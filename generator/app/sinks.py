import json
import logging
import sys
from typing import Protocol

import httpx

from .config import settings

logger = logging.getLogger(__name__)


class Sink(Protocol):
    async def start(self) -> None: ...
    async def send(self, events: list[dict]) -> None: ...
    async def stop(self) -> None: ...


class StdoutSink:
    async def start(self) -> None:
        pass

    async def send(self, events: list[dict]) -> None:
        for event in events:
            sys.stdout.write(json.dumps(event, ensure_ascii=False) + "\n")
        sys.stdout.flush()

    async def stop(self) -> None:
        pass


class HttpSink:
    def __init__(self, url: str):
        self.url = url
        self.client: httpx.AsyncClient | None = None

    async def start(self) -> None:
        self.client = httpx.AsyncClient(timeout=10)

    async def send(self, events: list[dict]) -> None:
        response = await self.client.post(self.url, json=events)
        response.raise_for_status()

    async def stop(self) -> None:
        if self.client:
            await self.client.aclose()


class KafkaSink:
    def __init__(self, bootstrap_servers: str, topic: str):
        self.bootstrap_servers = bootstrap_servers
        self.topic = topic
        self.producer = None

    async def start(self) -> None:
        from aiokafka import AIOKafkaProducer

        self.producer = AIOKafkaProducer(
            bootstrap_servers=self.bootstrap_servers,
            value_serializer=lambda v: json.dumps(v, ensure_ascii=False).encode(),
            key_serializer=lambda k: k.encode(),
        )
        await self.producer.start()

    async def send(self, events: list[dict]) -> None:
        for event in events:
            await self.producer.send(self.topic, value=event, key=event["order_id"])
        await self.producer.flush()

    async def stop(self) -> None:
        if self.producer:
            await self.producer.stop()


def build_sink() -> Sink:
    if settings.sink == "http":
        return HttpSink(settings.http_target_url)
    if settings.sink == "kafka":
        return KafkaSink(settings.kafka_bootstrap_servers, settings.kafka_topic)
    if settings.sink != "stdout":
        logger.warning("Unknown SINK=%s, falling back to stdout", settings.sink)
    return StdoutSink()
