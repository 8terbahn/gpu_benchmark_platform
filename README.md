# GPU Benchmark Platform

Enterprise-grade Kubernetes-native GPU benchmark platform built with Go, Crossplane,
and the Operator pattern. Enables product teams to self-serve GPU benchmark execution
through a single declarative YAML, with policy compliance and automated result capture.

Originally a Python FastAPI async benchmark backend; evolved into a full
**Platform Engineering reference implementation** demonstrating:

- Kubernetes Operator (Kubebuilder / controller-runtime) — Go
- Crossplane self-service control plane (XRD → Composition → Go Function)
- Policy-as-Code (Kyverno + CEL)
- Helm packaging + GitOps CI/CD (GitHub Actions, 6 stages)
- SRE-grade async backend (FastAPI + Redis queue + PostgreSQL + DLQ)

---

## Architecture

```
User
  │  kubectl apply -f benchmark-job.yaml
  ▼
┌────────────────────────────────────────────────────────────┐
│              Kyverno Admission Policies                     │
│   (GPU model allow-list, workload_type, labels)            │
└──────────────────────────┬─────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────┐
│         Crossplane Composition Engine                       │
│   XBenchmarkEnvironment XRD → Composition                  │
│   → function-benchmark-sizing (Go gRPC)                    │
│   Provisions: Namespace + ResourceQuota + BenchmarkJob CR   │
└──────────────────────────┬─────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────┐
│    BenchmarkJob Operator (controller-runtime / Kubebuilder) │
│                                                            │
│  BenchmarkJobReconciler                                    │
│  ┌─────────────────────────────────────────┐              │
│  │ Phase state machine:                    │              │
│  │  Pending → Submitted → Running          │              │
│  │                      → Completed        │              │
│  │                      → Failed           │              │
│  │  Finalizer: cancel backend job on del   │              │
│  │  Auto-retry with RequeueAfter backoff   │              │
│  └─────────────────────────────────────────┘              │
└──────────────────────────┬─────────────────────────────────┘
                           │ HTTP
┌──────────────────────────▼─────────────────────────────────┐
│       Custom Crossplane Provider (provider-gpu-benchmark)   │
│   GPUBenchmarkJob (Managed Resource) → managed.Reconciler  │
│   Observe / Create / Delete → Python FastAPI Backend       │
└──────────────────────────┬─────────────────────────────────┘
                           │ HTTP
┌──────────────────────────▼─────────────────────────────────┐
│         Python FastAPI Backend                             │
│   FastAPI ←→ Redis Queue ←→ Worker ←→ PostgreSQL           │
│            ↕                                               │
│   GPU Engine: NVIDIA Runtime Adapter + Simulated fallback  │
└─────────────────────────────────────────────────────────────┘
```

---

## Technology Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Control Plane** | Crossplane (XRD + Composition + Go Function) | Self-service benchmark environment API |
| **Provider** | Crossplane Provider (Go) | Crossplane-native external resource management |
| **Operator** | Kubebuilder + controller-runtime (Go) | BenchmarkJob lifecycle + reconciliation |
| **Policy** | Kyverno (ClusterPolicy + CEL) | Admission-time compliance enforcement |
| **Packaging / CD** | Helm + GitHub Actions | 6-stage CI: lint → test → build → scan → chart → E2E |
| **API** | Python + FastAPI (async) | Benchmark job REST API |
| **Queue** | Redis | Async dispatch + Dead Letter Queue |
| **Database** | PostgreSQL + Alembic | Job metadata and results persistence |
| **Worker** | Python | NVIDIA runtime + simulated GPU engine |
| **Runtime** | Kubernetes (Kind locally, any K8s in production) | Container orchestration |

---

## Repository Structure

```
gpu_benchmark_platform/
│
├── operator/                          ← Kubernetes Operator (Go) — primary component
│   ├── api/v1alpha1/
│   │   ├── groupversion_info.go       # API group: benchmarks.gpu-platform.io
│   │   ├── benchmarkjob_types.go      # BenchmarkJob CRD spec / status
│   │   └── zz_generated.deepcopy.go  # Generated deep-copy methods
│   ├── controllers/
│   │   ├── benchmarkjob_controller.go # Reconcile: state machine + finalizer + HTTP client
│   │   └── suite_test.go              # envtest integration tests (mock HTTP server)
│   ├── cmd/main.go                    # Operator entry point (controller-manager)
│   ├── go.mod
│   └── Dockerfile                     # Multi-stage: alpine builder → distroless nonroot
│
├── provider/                          ← Custom Crossplane Provider (Go)
│   ├── api/v1alpha1/                  # Managed Resource (GPUBenchmarkJob) + ProviderConfig
│   ├── internal/controller/           # External client (Observe/Create/Delete)
│   ├── cmd/provider/main.go           # Provider entry point
│   ├── package/crossplane.yaml        # Provider package manifest
│   ├── go.mod
│   └── Dockerfile                     # Multi-stage builder
│
├── crossplane/                        ← Crossplane Control Plane
│   ├── xrd.yaml                       # XBenchmarkEnvironment XRD
│   ├── composition.yaml               # Namespace + ResourceQuota + BenchmarkJob CR
│   ├── provider-config.yaml           # Kubernetes ProviderConfig (workload identity)
│   └── functions/benchmark-sizing/
│       ├── main.go                    # Go gRPC Crossplane Function: validates + sizes
│       └── go.mod
│
├── policies/kyverno/                  ← Policy-as-Code
│   └── benchmark-policies.yaml        # GPU allow-list, workload enum, label requirements
│
├── charts/benchmark-platform/        ← Helm Chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml            # Operator + API + Worker Deployments + Service
│       ├── provider-deployment.yaml   # Provider Deployment
│       └── rbac.yaml                  # ServiceAccount, ClusterRole, ClusterRoleBinding
│
├── config/crd/bases/                  ← CRD Manifest
│   └── benchmarks.gpu-platform.io_benchmarkjobs.yaml
│
├── backend/                           ← Python FastAPI Backend (unchanged)
│   ├── app/
│   │   ├── api/benchmarks.py          # REST: POST /benchmarks, GET /benchmarks/{id}
│   │   ├── services/benchmark_service.py  # Idempotent job creation, state transitions
│   │   ├── services/gpu_engine.py     # NVIDIA runtime + simulated engine
│   │   ├── services/queue_service.py  # Redis queue + DLQ
│   │   ├── services/sustainability.py # Carbon footprint calculation
│   │   ├── workers/benchmark_worker.py # Async worker runtime
│   │   └── models/                    # SQLAlchemy models
│   ├── alembic/                       # Database migrations
│   └── tests/                         # Pytest suite
│
├── docs/
│   ├── architecture.md               # System architecture + design decisions
│   └── developer-guide.md            # Self-service guide for product teams
│
├── .github/workflows/ci.yml          ← CI/CD (lint→test→build→scan→chart→E2E)
├── docker-compose.yml                ← Local dev (PostgreSQL + Redis)
├── Dockerfile                        ← Python API/worker image
└── README.md
```

---

## Quick Start

### Option A — Local Python backend (no cluster required)

```bash
docker compose up --build
cd backend && alembic upgrade head

# Submit a benchmark via REST API
curl -X POST http://localhost:8000/benchmarks \
  -H "Content-Type: application/json" \
  -d '{"gpu_model": "H100", "workload_type": "inference", "batch_size": 32}'

# Poll for result
curl http://localhost:8000/benchmarks/<job_id>
```

### Option B — Kubernetes Operator (Kind cluster)

```bash
# 1. Create cluster
kind create cluster --name benchmark-platform

# 2. Install CRDs
kubectl apply -f config/crd/bases/

# 3. Install Kyverno (policy engine)
helm repo add kyverno https://kyverno.github.io/kyverno/
helm install kyverno kyverno/kyverno -n kyverno --create-namespace --wait

# 4. Deploy the platform
helm install benchmark-platform charts/benchmark-platform \
  --namespace gpu-benchmark --create-namespace

# 5. Apply policies + Crossplane manifests
kubectl apply -f policies/kyverno/
kubectl apply -f crossplane/   # requires Crossplane installed

# 6. Submit a benchmark declaratively
kubectl apply -f - <<EOF
apiVersion: benchmarks.gpu-platform.io/v1alpha1
kind: BenchmarkJob
metadata:
  name: h100-inference
  namespace: gpu-benchmark
  labels:
    gpu-platform.io/cost-center: CC-0001
    gpu-platform.io/owner: ml-team
spec:
  gpuModel: H100
  workloadType: inference
  batchSize: 32
EOF

# 7. Watch the lifecycle
kubectl get benchmarkjob -n gpu-benchmark -w
```

### Option C — Self-Service via Crossplane Claim

See [docs/developer-guide.md](docs/developer-guide.md).

### Option D — Self-Service via Provider Managed Resource

Directly submit a `GPUBenchmarkJob` Managed Resource to the Crossplane Provider:

```yaml
apiVersion: gpu.platform.io/v1alpha1
kind: GPUBenchmarkJob
metadata:
  name: provider-test-h100
spec:
  providerConfigRef:
    name: default
  forProvider:
    gpuModel: H100
    workloadType: inference
    batchSize: 32
```

---

## CI/CD Pipeline

`.github/workflows/ci.yml` runs on every push:

| Stage | What it does |
|-------|-------------|
| **Lint** | `golangci-lint` (Go operator + Crossplane function) + `ruff` (Python) |
| **Test** | `envtest` integration tests + Python `pytest` + 70% coverage gate |
| **Build** | Docker multi-stage: operator (distroless), API/worker images |
| **Scan** | Trivy (CRITICAL/HIGH blocks merge) + SARIF upload to GitHub Security |
| **Chart** | `helm lint` + template render |
| **E2E** | Kind cluster: CRDs, chart deploy, submit BenchmarkJob, assert `Completed` phase |

---

## Documentation

| Document | Purpose |
|----------|---------|
| [architecture.md](docs/architecture.md) | Full system architecture and design decisions |
| [developer-guide.md](docs/developer-guide.md) | Self-service guide: BenchmarkJob CR + Crossplane Claim |
| [aws_deployment.md](docs/aws_deployment.md) | AWS reference architecture (ALB + ECS/Fargate + RDS) |

---

## Design Highlights

- **Declarative self-healing**: `BenchmarkJob` CRs are level-triggered; the operator
  recovers automatically from pod restarts, API unavailability, and network partitions.
- **Finalizer-based cleanup**: backend jobs are cancelled on CR deletion — no orphaned
  work items in the Redis queue.
- **Zero-invasive layering**: the Go control plane calls the existing Python REST API
  without modifying the backend. Execution and scheduling state are fully decoupled.
- **Compliance-by-default**: Kyverno enforces GPU model allow-lists and cost-center
  labels at admission time — before any resource reaches etcd.
- **Idempotent submission**: `namespace/name` used as idempotency key — safe to retry
  on any transient failure without duplicate jobs.
- **GitOps-ready**: Helm chart + 6-stage GitHub Actions CI → reproducible, auditable deployments.
