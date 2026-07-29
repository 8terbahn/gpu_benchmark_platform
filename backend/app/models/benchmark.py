import enum
import uuid
from datetime import datetime

from sqlalchemy import DateTime, Enum, ForeignKey, Index, Integer, String
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship
from sqlalchemy.sql import func

from app.database.base import Base


class BenchmarkStatus(str, enum.Enum):
    queued = "queued"
    running = "running"
    completed = "completed"
    failed = "failed"


class Benchmark(Base):
    __tablename__ = "benchmarks"
    __table_args__ = (
        Index("ix_benchmarks_gpu_model_created_at", "gpu_model_id", "created_at"),
        Index("ix_benchmarks_idempotency_key", "idempotency_key", unique=True),
    )

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    gpu_model_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("gpu_models.id"), index=True)
    workload_type: Mapped[str] = mapped_column(String(64), nullable=False)
    batch_size: Mapped[int] = mapped_column(Integer, nullable=False)
    status: Mapped[BenchmarkStatus] = mapped_column(Enum(BenchmarkStatus), default=BenchmarkStatus.queued)
    idempotency_key: Mapped[str] = mapped_column(String(128), nullable=False)
    attempt_count: Mapped[int] = mapped_column(Integer, nullable=False, default=0)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), index=True)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now(), onupdate=func.now(), nullable=False
    )

    gpu_model = relationship("GPUModel", back_populates="benchmarks", lazy="joined")
    result = relationship("BenchmarkResult", back_populates="benchmark", uselist=False, lazy="selectin")
