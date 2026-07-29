import uuid
from datetime import datetime

from sqlalchemy import JSON, DateTime, Float, ForeignKey, Index
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship
from sqlalchemy.sql import func

from app.database.base import Base


class BenchmarkResult(Base):
    __tablename__ = "benchmark_results"
    __table_args__ = (Index("ix_results_created_at", "created_at"),)

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    benchmark_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("benchmarks.id"), unique=True, index=True)
    throughput: Mapped[float] = mapped_column(Float, nullable=False)
    latency_ms: Mapped[float] = mapped_column(Float, nullable=False)
    memory_bandwidth_gbps: Mapped[float] = mapped_column(Float, nullable=False)
    raw_metrics: Mapped[dict] = mapped_column(JSON, default=dict, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), nullable=False)

    benchmark = relationship("Benchmark", back_populates="result", lazy="joined")
