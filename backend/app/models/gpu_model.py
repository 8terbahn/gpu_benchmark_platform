import uuid
from datetime import datetime

from sqlalchemy import DateTime, String
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship
from sqlalchemy.sql import func

from app.database.base import Base


class GPUModel(Base):
    __tablename__ = "gpu_models"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    model_name: Mapped[str] = mapped_column(String(64), unique=True, index=True, nullable=False)
    vendor: Mapped[str] = mapped_column(String(32), default="nvidia", nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), nullable=False)

    benchmarks = relationship("Benchmark", back_populates="gpu_model", lazy="selectin")
