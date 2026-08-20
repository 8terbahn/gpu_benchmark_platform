# GPU Benchmark Platform — Architecture

## System Overview

```
User / Product Team
  │  kubectl apply -f benchmark-env.yaml   (or: kubectl apply -f benchmark-job.yaml)
  ▼
┌────────────────────────────────────────────────────────────┐
│                Kyverno Admission Policies                   │
│   (GPU model allow-list, workload_type enum, label check)  │
└──────────────────────────┬─────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────┐
│            Crossplane Composition Engine (optional)         │
│   XBenchmarkEnvironment XRD → Composition                  │
│   → function-benchmark-sizing (Go gRPC)                    │
│   Provisions: Namespace, ResourceQuota, BenchmarkJob CR     │
└──────────────────────────┬─────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────┐
│         BenchmarkJob Operator (Go / controller-runtime)     │
│                                                            │
│  BenchmarkJobReconciler                                    │
│  ┌────────────────────────────────────────┐               │
│  │ Phase state machine:                   │               │
│  │  Pending → Submitted (POST /benchmarks) │              │
│  │         → Running   (poll GET)         │               │
│  │         → Completed (write results)    │               │
│  │         → Failed    (max retries)      │               │
│  │  Finalizer: cancel backend job on del  │               │
│  │  Auto-retry: RequeueAfter on errors    │               │
│  └────────────────────────────────────────┘               │
└──────────────────────────┬─────────────────────────────────┘
                           │ HTTP (REST)
┌──────────────────────────▼─────────────────────────────────┐
│         Python FastAPI Backend (unchanged)                  │
│                                                            │
│  POST /benchmarks   → validates + writes to PostgreSQL      │
│                     → pushes to Redis queue                │
│  GET  /benchmarks/{id} → returns current status + result   │
│                                                            │
│  Worker: Redis queue → GPU Engine (NVIDIA or simulated)    │
│         → persists results to PostgreSQL                   │
└─────────────────────────────────────────────────────────────┘
```

## Component Summary

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Control Plane** | Crossplane (XRD + Composition + Go Function) | Self-service benchmark environment API |
| **Operator** | Kubebuilder + controller-runtime (Go) | BenchmarkJob lifecycle management |
| **Policy** | Kyverno (ClusterPolicy + CEL) | Admission-time compliance |
| **Packaging / CD** | Helm + GitHub Actions | 6-stage CI: lint → test → build → scan → chart → E2E |
| **API Backend** | Python + FastAPI | Benchmark job execution REST API |
| **Queue** | Redis | Async job dispatch + DLQ |
| **Database** | PostgreSQL | Job metadata + results persistence |
| **Worker** | Python | GPU benchmark execution engine |

## Design Principles

- **Declarative self-healing**: `BenchmarkJob` CRs are level-triggered; the operator
  re-reconciles after pod restarts, network partitions, and API unavailability.
- **Finalizer-based cleanup**: backend jobs are cancelled on CR deletion — no orphaned
  work items in the Redis queue.
- **Idempotent submission**: the operator uses `namespace/name` as the idempotency key
  when calling the Python API — safe to retry on any transient failure.
- **Zero-invasive layering**: the Go control plane calls the existing Python REST API
  without modifying the backend. Execution logic and scheduling state are fully decoupled.
- **Compliance by default**: Kyverno enforces GPU model allow-lists and cost-center labels
  at admission time, before any resource reaches etcd.
