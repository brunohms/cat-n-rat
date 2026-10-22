import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    # Event destination: "stdout", "http" or "kafka"
    sink: str = os.getenv("SINK", "stdout")
    # Events per second for the continuous stream (0 disables it)
    events_per_second: float = float(os.getenv("EVENTS_PER_SECOND", "1"))
    # Used when SINK=http
    http_target_url: str = os.getenv("HTTP_TARGET_URL", "http://localhost:9000/events")
    # Used when SINK=kafka
    kafka_bootstrap_servers: str = os.getenv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
    kafka_topic: str = os.getenv("KAFKA_TOPIC", "purchases")
    # Faker locale
    locale: str = os.getenv("FAKER_LOCALE", "pt_BR")
    # Optional seed for reproducible generation
    seed: int | None = int(os.environ["SEED"]) if os.getenv("SEED") else None
    # Maximum number of events per request
    max_batch: int = int(os.getenv("MAX_BATCH", "10000"))


settings = Settings()
