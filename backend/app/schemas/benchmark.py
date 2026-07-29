from datetime import datetime
from uuid import UUID

from pydantic import BaseModel, Field


class BenchmarkCreateRequest(BaseModel):
    gpu_model: str = Field(..., examples=["H100", "MI300X"])
    workload_type: str = Field(..., examples=["training", "inference"])
    batch_size: int = Field(..., gt=0)
    idempotency_key: str | None = Field(default=None, max_length=128)


class BenchmarkCreateResponse(BaseModel):
    job_id: UUID
    status: str = "queued"
    submitted_at: datetime


class BenchmarkResultPayload(BaseModel):
    throughput: float
    latency_ms: float
    memory_bandwidth_gbps: float
    raw_metrics: dict[str, float | str | int]


class BenchmarkStatusResponse(BaseModel):
    job_id: UUID
    gpu_model: str
    workload_type: str
    batch_size: int
    status: str
    submitted_at: datetime
    updated_at: datetime | None = None
    result: BenchmarkResultPayload | None = None


class BenchmarkHistoryItem(BaseModel):
    job_id: UUID
    status: str
    workload_type: str
    batch_size: int
    submitted_at: datetime
    throughput: float | None = None
    latency_ms: float | None = None
