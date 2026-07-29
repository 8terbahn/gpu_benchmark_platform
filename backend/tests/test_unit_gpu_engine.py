import pytest

from app.services.gpu_engine import SimulatedGPUBenchmarkEngine


@pytest.mark.asyncio
async def test_gpu_engine_returns_positive_metrics() -> None:
    engine = SimulatedGPUBenchmarkEngine()
    result = await engine.run(gpu_model="H100", workload_type="inference", batch_size=8)

    assert result.throughput > 0
    assert result.latency_ms > 0
    assert result.memory_bandwidth_gbps > 0
