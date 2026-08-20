# Self-Service Developer Guide

This guide shows product teams how to submit GPU benchmark jobs declaratively
using the Kubernetes-native control plane.

---

## Option A — Direct BenchmarkJob CR (quickest)

Submit a single `BenchmarkJob` resource. The operator handles submission,
polling, retries, and result capture automatically.

```yaml
apiVersion: benchmarks.gpu-platform.io/v1alpha1
kind: BenchmarkJob
metadata:
  name: h100-inference-bench
  namespace: my-team
  labels:
    gpu-platform.io/cost-center: CC-4210   # required — Kyverno enforced
    gpu-platform.io/owner: ml-team         # required — Kyverno enforced
spec:
  gpuModel: H100          # T4 | V100 | A100 | H100 | L40S | RTX4090
  workloadType: inference  # inference | training
  batchSize: 32
  maxRetries: 3            # optional, default 3
```

```bash
kubectl apply -f benchmark-job.yaml

# Watch progress
kubectl get benchmarkjob h100-inference-bench -w

# Check results when Completed
kubectl get benchmarkjob h100-inference-bench -o jsonpath='{.status}'
```

Expected output progression:

| Phase | Meaning |
|-------|---------|
| `Pending` | CR created, operator about to submit to backend |
| `Submitted` | Job accepted by Python API, queued in Redis |
| `Running` | Worker is executing the benchmark |
| `Completed` | Results written to status; throughput/latency available |
| `Failed` | Max retries exhausted |

---

## Option B — Self-Service via Crossplane Claim (recommended for teams)

Submit a `BenchmarkEnvironment` Claim. Crossplane provisions a dedicated
Namespace, ResourceQuota, and BenchmarkJob CR automatically.

```yaml
apiVersion: platform.gpu-platform.io/v1alpha1
kind: BenchmarkEnvironment
metadata:
  name: my-h100-benchmark
  namespace: my-team
spec:
  gpuModel: H100
  workloadType: training
  batchSize: 64
  costCenter: CC-4210
  owner: ml-team
  maxRetries: 3
```

```bash
kubectl apply -f benchmark-environment.yaml

# Watch Crossplane provision the environment
kubectl get benchmarkenvironment my-h100-benchmark -w

# Once ready, check the BenchmarkJob in the provisioned namespace
kubectl get benchmarkjob -n benchmark-ml-team
```

---

## Reading Results

```bash
# Short summary
kubectl get benchmarkjob <name> -n <namespace>

# Full results
kubectl get benchmarkjob <name> -n <namespace> -o yaml
```

Key status fields:

| Field | Description |
|-------|-------------|
| `status.phase` | Current lifecycle phase |
| `status.jobID` | UUID from the Python backend |
| `status.throughput` | Measured samples/second |
| `status.latencyMs` | Measured latency in ms |
| `status.memoryBandwidthGbps` | GPU memory bandwidth in GB/s |
| `status.startTime` | When submission occurred |
| `status.completionTime` | When the job finished |

---

## Troubleshooting

**Job stuck in `Pending`?**
```bash
kubectl describe benchmarkjob <name> -n <namespace>
kubectl logs -n gpu-benchmark -l app.kubernetes.io/component=operator
```

**Kyverno policy violation?**
```bash
kubectl get policyreport -n <namespace>
```

Common rejections:
- `gpuModel` not in allow-list — use one of: T4, V100, A100, H100, L40S, RTX4090
- Missing `gpu-platform.io/cost-center` label (must match `CC-NNNN`)
- Missing `gpu-platform.io/owner` label
