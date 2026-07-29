from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.config import settings
from app.database.session import get_session
from app.schemas.benchmark import (
    BenchmarkCreateRequest,
    BenchmarkCreateResponse,
    BenchmarkHistoryItem,
    BenchmarkStatusResponse,
)
from app.services.benchmark_service import benchmark_service
from app.services.queue_service import queue_service

router = APIRouter(prefix="/benchmarks", tags=["benchmarks"])


@router.post("", response_model=BenchmarkCreateResponse, status_code=status.HTTP_202_ACCEPTED)
async def create_benchmark_job(
    payload: BenchmarkCreateRequest,
    session: AsyncSession = Depends(get_session),
) -> BenchmarkCreateResponse:
    benchmark, response, created = await benchmark_service.create_benchmark(session, payload)
    if created:
        await queue_service.enqueue(
            {
                "benchmark_id": str(benchmark.id),
                "gpu_model": payload.gpu_model,
                "workload_type": payload.workload_type,
                "batch_size": payload.batch_size,
                "attempt": 0,
            }
        )
    return response


@router.get("/gpu/{gpu_model}", response_model=list[BenchmarkHistoryItem])
async def get_benchmark_history(
    gpu_model: str,
    limit: int = Query(default=settings.default_history_limit, ge=1, le=500),
    session: AsyncSession = Depends(get_session),
) -> list[BenchmarkHistoryItem]:
    return await benchmark_service.get_gpu_history(session, gpu_model, limit)


@router.get("/{job_id}", response_model=BenchmarkStatusResponse)
async def get_benchmark_status(
    job_id: UUID,
    session: AsyncSession = Depends(get_session),
) -> BenchmarkStatusResponse:
    result = await benchmark_service.get_benchmark(session, job_id)
    if not result:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Benchmark job not found")
    return result
