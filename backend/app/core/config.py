from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "GPU Benchmark API"
    environment: str = "production"
    log_level: str = "INFO"

    database_url: str = "postgresql+asyncpg://postgres:postgres@postgres:5432/benchmark_db"
    redis_url: str = "redis://redis:6379/0"
    redis_queue_name: str = "benchmark:jobs"
    redis_dlq_name: str = "benchmark:jobs:dlq"

    worker_poll_seconds: float = 0.2
    worker_max_retries: int = 3

    default_history_limit: int = Field(default=100, ge=1, le=500)

    otel_enabled: bool = False
    otel_service_name: str = "gpu-benchmark-api"


settings = Settings()
