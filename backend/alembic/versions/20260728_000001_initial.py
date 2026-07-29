"""initial schema

Revision ID: 20260728_000001
Revises: 
Create Date: 2026-07-28
"""

from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

revision = "20260728_000001"
down_revision = None
branch_labels = None
depends_on = None

benchmark_status = sa.Enum("queued", "running", "completed", "failed", name="benchmarkstatus")


def upgrade() -> None:
    op.create_table(
        "gpu_models",
        sa.Column("id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("model_name", sa.String(length=64), nullable=False),
        sa.Column("vendor", sa.String(length=32), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
    )
    op.create_index("ix_gpu_models_model_name", "gpu_models", ["model_name"], unique=True)

    benchmark_status.create(op.get_bind(), checkfirst=True)
    op.create_table(
        "benchmarks",
        sa.Column("id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("gpu_model_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("gpu_models.id"), nullable=False),
        sa.Column("workload_type", sa.String(length=64), nullable=False),
        sa.Column("batch_size", sa.Integer(), nullable=False),
        sa.Column("status", benchmark_status, nullable=False),
        sa.Column("idempotency_key", sa.String(length=128), nullable=False),
        sa.Column("attempt_count", sa.Integer(), nullable=False, server_default="0"),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
    )
    op.create_index("ix_benchmarks_gpu_model_id", "benchmarks", ["gpu_model_id"])
    op.create_index("ix_benchmarks_created_at", "benchmarks", ["created_at"])
    op.create_index("ix_benchmarks_gpu_model_created_at", "benchmarks", ["gpu_model_id", "created_at"])
    op.create_index("ix_benchmarks_idempotency_key", "benchmarks", ["idempotency_key"], unique=True)

    op.create_table(
        "benchmark_results",
        sa.Column("id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("benchmark_id", postgresql.UUID(as_uuid=True), sa.ForeignKey("benchmarks.id"), nullable=False),
        sa.Column("throughput", sa.Float(), nullable=False),
        sa.Column("latency_ms", sa.Float(), nullable=False),
        sa.Column("memory_bandwidth_gbps", sa.Float(), nullable=False),
        sa.Column("raw_metrics", sa.JSON(), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
    )
    op.create_index("ix_benchmark_results_benchmark_id", "benchmark_results", ["benchmark_id"], unique=True)
    op.create_index("ix_results_created_at", "benchmark_results", ["created_at"])


def downgrade() -> None:
    op.drop_index("ix_results_created_at", table_name="benchmark_results")
    op.drop_index("ix_benchmark_results_benchmark_id", table_name="benchmark_results")
    op.drop_table("benchmark_results")

    op.drop_index("ix_benchmarks_idempotency_key", table_name="benchmarks")
    op.drop_index("ix_benchmarks_gpu_model_created_at", table_name="benchmarks")
    op.drop_index("ix_benchmarks_created_at", table_name="benchmarks")
    op.drop_index("ix_benchmarks_gpu_model_id", table_name="benchmarks")
    op.drop_table("benchmarks")

    op.drop_index("ix_gpu_models_model_name", table_name="gpu_models")
    op.drop_table("gpu_models")

    benchmark_status.drop(op.get_bind(), checkfirst=True)
