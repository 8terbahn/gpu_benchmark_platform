from datetime import datetime, timezone
from types import SimpleNamespace
from uuid import uuid4

from fastapi.testclient import TestClient

from app.main import app
from app.schemas.benchmark import (
    BenchmarkCreateResponse,
    BenchmarkHistoryItem,
    BenchmarkResultPayload,
    BenchmarkStatusResponse,
)
from app.services.benchmark_service import benchmark_service
from app.services.queue_service import queue_service

client = TestClient(app)


def test_health_endpoint() -> None:
    response = client.get("/")
    assert response.status_code == 200
    assert response.json()["status"] == "healthy"


def test_create_benchmark_endpoint(monkeypatch) -> None:
    job_id = uuid4()
    now = datetime.now(timezone.utc)

    async def mock_create_benchmark(session, payload):
        benchmark = SimpleNamespace(id=job_id)
        response = BenchmarkCreateResponse(job_id=job_id, status="queued", submitted_at=now)
        return benchmark, response, True

    async def mock_enqueue(payload):
        return None

    monkeypatch.setattr(benchmark_service, "create_benchmark", mock_create_benchmark)
    monkeypatch.setattr(queue_service, "enqueue", mock_enqueue)

    response = client.post(
        "/benchmarks",
        json={"gpu_model": "H100", "workload_type": "training", "batch_size": 16},
    )

    assert response.status_code == 202
    assert response.json()["status"] == "queued"


def test_get_benchmark_status_endpoint(monkeypatch) -> None:
    job_id = uuid4()

    async def mock_get_benchmark(session, _job_id):
        return BenchmarkStatusResponse(
            job_id=job_id,
            gpu_model="H100",
            workload_type="training",
            batch_size=16,
            status="completed",
            submitted_at=datetime.now(timezone.utc),
            result=BenchmarkResultPayload(
                throughput=100.0,
                latency_ms=10.0,
                memory_bandwidth_gbps=1200.0,
                raw_metrics={"engine": "sim", "attempt": 1},
            ),
        )

    monkeypatch.setattr(benchmark_service, "get_benchmark", mock_get_benchmark)

    response = client.get(f"/benchmarks/{job_id}")
    assert response.status_code == 200
    assert response.json()["status"] == "completed"


def test_get_benchmark_history_endpoint(monkeypatch) -> None:
    async def mock_get_history(session, gpu_model, limit=100):
        return [
            BenchmarkHistoryItem(
                job_id=uuid4(),
                status="completed",
                workload_type="inference",
                batch_size=8,
                submitted_at=datetime.now(timezone.utc),
                throughput=220.0,
                latency_ms=6.5,
            )
        ]

    monkeypatch.setattr(benchmark_service, "get_gpu_history", mock_get_history)

    response = client.get("/benchmarks/gpu/H100")
    assert response.status_code == 200
    assert len(response.json()) == 1
