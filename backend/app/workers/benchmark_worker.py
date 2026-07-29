import asyncio
import logging
from uuid import UUID

from app.core.config import settings
from app.core.logging import configure_logging
from app.database.session import SessionLocal
from app.services.benchmark_service import benchmark_service
from app.services.gpu_engine import NvidiaRuntimeAdapter, SimulatedGPUBenchmarkEngine
from app.services.queue_service import queue_service

logger = logging.getLogger(__name__)


class BenchmarkWorker:
    def __init__(self) -> None:
        self.nvidia_engine = NvidiaRuntimeAdapter()
        self.sim_engine = SimulatedGPUBenchmarkEngine()

    async def run_forever(self) -> None:
        while True:
            job = await queue_service.dequeue(timeout=5)
            if not job:
                await asyncio.sleep(settings.worker_poll_seconds)
                continue
            await self._process_job(job)

    async def _process_job(self, job: dict) -> None:
        benchmark_id = UUID(job["benchmark_id"])
        attempt = int(job.get("attempt", 0))

        async with SessionLocal() as session:
            try:
                await benchmark_service.mark_running(session, benchmark_id)
                engine = self.nvidia_engine if await self.nvidia_engine.runtime_available() else self.sim_engine
                result = await engine.run(
                    gpu_model=job["gpu_model"],
                    workload_type=job["workload_type"],
                    batch_size=int(job["batch_size"]),
                )
                await benchmark_service.complete_benchmark(
                    session=session,
                    benchmark_id=benchmark_id,
                    throughput=result.throughput,
                    latency_ms=result.latency_ms,
                    memory_bandwidth_gbps=result.memory_bandwidth_gbps,
                    raw_metrics={
                        "engine": engine.__class__.__name__,
                        "workload": job["workload_type"],
                        "attempt": attempt + 1,
                    },
                )
            except Exception as exc:
                logger.exception("worker_job_failed", extra={"benchmark_id": str(benchmark_id), "attempt": attempt})
                if attempt + 1 < settings.worker_max_retries:
                    await queue_service.enqueue_retry(job)
                else:
                    await queue_service.move_to_dlq(job, reason=str(exc))
                    await benchmark_service.mark_failed(session, benchmark_id)


async def main() -> None:
    configure_logging()
    worker = BenchmarkWorker()
    await worker.run_forever()


if __name__ == "__main__":
    asyncio.run(main())
