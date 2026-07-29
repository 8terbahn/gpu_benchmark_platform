from app.models.benchmark import Benchmark
from app.models.benchmark_result import BenchmarkResult
from app.models.gpu_model import GPUModel


def test_core_tables_exist() -> None:
    assert GPUModel.__tablename__ == "gpu_models"
    assert Benchmark.__tablename__ == "benchmarks"
    assert BenchmarkResult.__tablename__ == "benchmark_results"


def test_optimized_indexes_exist() -> None:
    index_names = {index.name for index in Benchmark.__table__.indexes}
    assert "ix_benchmarks_gpu_model_created_at" in index_names
    assert "ix_benchmarks_idempotency_key" in index_names
