import asyncio
from dataclasses import dataclass
from random import uniform
from typing import Protocol


@dataclass(slots=True)
class EngineResult:
    throughput: float
    latency_ms: float
    memory_bandwidth_gbps: float


class BenchmarkEngine(Protocol):
    async def run(self, gpu_model: str, workload_type: str, batch_size: int) -> EngineResult:
        ...


class SimulatedGPUBenchmarkEngine:
    async def run(self, gpu_model: str, workload_type: str, batch_size: int) -> EngineResult:
        base = 2200.0 if workload_type == "inference" else 1300.0
        architecture_multiplier = 1.25 if gpu_model.upper().startswith("H") else 1.1
        throughput = (base * architecture_multiplier) / max(batch_size, 1)
        latency_ms = uniform(3.0, 12.0) + batch_size / 16
        memory_bandwidth_gbps = uniform(600, 1500)
        return EngineResult(
            throughput=round(throughput, 2),
            latency_ms=round(latency_ms, 3),
            memory_bandwidth_gbps=round(memory_bandwidth_gbps, 2),
        )


class NvidiaRuntimeAdapter:
    async def runtime_available(self) -> bool:
        try:
            process = await asyncio.create_subprocess_exec(
                "nvidia-smi",
                "--query-gpu=name",
                "--format=csv,noheader",
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            await process.communicate()
            return process.returncode == 0
        except Exception:
            return False

    async def run(self, gpu_model: str, workload_type: str, batch_size: int) -> EngineResult:
        available = await self.runtime_available()
        if not available:
            raise RuntimeError("nvidia runtime unavailable")
        process = await asyncio.create_subprocess_exec(
            "nvidia-smi",
            "--query-gpu=utilization.gpu,memory.used",
            "--format=csv,noheader,nounits",
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, _ = await process.communicate()
        utilization, memory_used = [float(x.strip()) for x in stdout.decode().split(",")[:2]]
        return EngineResult(
            throughput=round(max(1.0, utilization) * 12.5 / max(batch_size, 1), 2),
            latency_ms=round(max(1.0, 120 - utilization) / 10, 3),
            memory_bandwidth_gbps=round(max(200.0, memory_used / 10), 2),
        )
