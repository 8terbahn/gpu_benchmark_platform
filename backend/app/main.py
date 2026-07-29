from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.benchmarks import router as benchmarks_router
from app.api.health import router as health_router
from app.core.config import settings
from app.core.logging import configure_logging
from app.core.telemetry import configure_telemetry
from app.services.queue_service import queue_service


@asynccontextmanager
async def lifespan(_: FastAPI):
    configure_logging()
    configure_telemetry()
    yield
    await queue_service.close()


app = FastAPI(title=settings.app_name, lifespan=lifespan)
app.include_router(health_router)
app.include_router(benchmarks_router)
