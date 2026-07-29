from hashlib import sha256
from uuid import UUID

from sqlalchemy import Select, select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import joinedload

from app.models import Benchmark, BenchmarkResult, BenchmarkStatus, GPUModel
from app.schemas.benchmark import (
    BenchmarkCreateRequest,
    BenchmarkCreateResponse,
    BenchmarkHistoryItem,
    BenchmarkResultPayload,
    BenchmarkStatusResponse,
)


class BenchmarkService:
    async def create_benchmark(
        self, session: AsyncSession, payload: BenchmarkCreateRequest
    ) -> tuple[Benchmark, BenchmarkCreateResponse, bool]:
        idempotency_key = payload.idempotency_key or self._build_idempotency_key(payload)

        existing = await self._find_by_idempotency_key(session, idempotency_key)
        if existing:
            return (
                existing,
                BenchmarkCreateResponse(
                    job_id=existing.id,
                    status=existing.status.value,
                    submitted_at=existing.created_at,
                ),
                False,
            )

        gpu_model = await self._get_or_create_gpu_model(session, payload.gpu_model)
        benchmark = Benchmark(
            gpu_model_id=gpu_model.id,
            workload_type=payload.workload_type,
            batch_size=payload.batch_size,
            status=BenchmarkStatus.queued,
            idempotency_key=idempotency_key,
        )
        session.add(benchmark)

        try:
            await session.commit()
        except IntegrityError:
            await session.rollback()
            existing = await self._find_by_idempotency_key(session, idempotency_key)
            if not existing:
                raise
            return (
                existing,
                BenchmarkCreateResponse(
                    job_id=existing.id,
                    status=existing.status.value,
                    submitted_at=existing.created_at,
                ),
                False,
            )

        await session.refresh(benchmark)
        return (
            benchmark,
            BenchmarkCreateResponse(
                job_id=benchmark.id,
                status=benchmark.status.value,
                submitted_at=benchmark.created_at,
            ),
            True,
        )

    async def get_benchmark(self, session: AsyncSession, job_id: UUID) -> BenchmarkStatusResponse | None:
        query: Select[tuple[Benchmark]] = (
            select(Benchmark)
            .where(Benchmark.id == job_id)
            .options(joinedload(Benchmark.gpu_model), joinedload(Benchmark.result))
        )
        record = (await session.execute(query)).scalar_one_or_none()
        if not record:
            return None
        result_payload = None
        if record.result:
            result_payload = BenchmarkResultPayload(
                throughput=record.result.throughput,
                latency_ms=record.result.latency_ms,
                memory_bandwidth_gbps=record.result.memory_bandwidth_gbps,
                raw_metrics=record.result.raw_metrics,
            )
        return BenchmarkStatusResponse(
            job_id=record.id,
            gpu_model=record.gpu_model.model_name,
            workload_type=record.workload_type,
            batch_size=record.batch_size,
            status=record.status.value,
            submitted_at=record.created_at,
            updated_at=record.updated_at,
            result=result_payload,
        )

    async def get_gpu_history(
        self, session: AsyncSession, gpu_model: str, limit: int = 100
    ) -> list[BenchmarkHistoryItem]:
        query: Select[tuple[Benchmark]] = (
            select(Benchmark)
            .join(GPUModel)
            .where(GPUModel.model_name == gpu_model)
            .options(joinedload(Benchmark.result))
            .order_by(Benchmark.created_at.desc())
            .limit(limit)
        )
        rows = (await session.execute(query)).scalars().all()
        return [
            BenchmarkHistoryItem(
                job_id=row.id,
                status=row.status.value,
                workload_type=row.workload_type,
                batch_size=row.batch_size,
                submitted_at=row.created_at,
                throughput=row.result.throughput if row.result else None,
                latency_ms=row.result.latency_ms if row.result else None,
            )
            for row in rows
        ]

    async def mark_running(self, session: AsyncSession, benchmark_id: UUID) -> Benchmark | None:
        benchmark = await session.get(Benchmark, benchmark_id)
        if not benchmark:
            return None
        benchmark.status = BenchmarkStatus.running
        benchmark.attempt_count += 1
        await session.commit()
        await session.refresh(benchmark)
        return benchmark

    async def complete_benchmark(
        self,
        session: AsyncSession,
        benchmark_id: UUID,
        throughput: float,
        latency_ms: float,
        memory_bandwidth_gbps: float,
        raw_metrics: dict[str, float | str | int],
    ) -> bool:
        benchmark = await session.get(Benchmark, benchmark_id)
        if not benchmark:
            return False
        benchmark.status = BenchmarkStatus.completed
        session.add(
            BenchmarkResult(
                benchmark_id=benchmark_id,
                throughput=throughput,
                latency_ms=latency_ms,
                memory_bandwidth_gbps=memory_bandwidth_gbps,
                raw_metrics=raw_metrics,
            )
        )
        await session.commit()
        return True

    async def mark_failed(self, session: AsyncSession, benchmark_id: UUID) -> None:
        benchmark = await session.get(Benchmark, benchmark_id)
        if benchmark:
            benchmark.status = BenchmarkStatus.failed
            await session.commit()

    async def _get_or_create_gpu_model(self, session: AsyncSession, model_name: str) -> GPUModel:
        result = await session.execute(select(GPUModel).where(GPUModel.model_name == model_name))
        gpu_model = result.scalar_one_or_none()
        if gpu_model:
            return gpu_model
        gpu_model = GPUModel(model_name=model_name)
        session.add(gpu_model)
        await session.flush()
        return gpu_model

    async def _find_by_idempotency_key(self, session: AsyncSession, idempotency_key: str) -> Benchmark | None:
        result = await session.execute(select(Benchmark).where(Benchmark.idempotency_key == idempotency_key))
        return result.scalar_one_or_none()

    def _build_idempotency_key(self, payload: BenchmarkCreateRequest) -> str:
        raw = f"{payload.gpu_model}:{payload.workload_type}:{payload.batch_size}"
        return sha256(raw.encode("utf-8")).hexdigest()


benchmark_service = BenchmarkService()
