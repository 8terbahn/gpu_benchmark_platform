# GPU Benchmark Platform

Enterprise-grade asynchronous backend for distributed GPU benchmark execution.

## System Architecture
`Client -> FastAPI API Gateway -> PostgreSQL (metadata/results) + Redis Queue -> Worker Service -> GPU Benchmark Engine`

## Backend Stack
- Python + FastAPI (async REST)
- Pydantic v2 schemas
- SQLAlchemy 2.0 + asyncpg
- Alembic migrations
- Redis queue
- Pytest test suite

## APIs
- `POST /benchmarks` create benchmark job (idempotent)
- `GET /benchmarks/{job_id}` fetch benchmark status/result
- `GET /benchmarks/gpu/{gpu_model}` query benchmark history

## Async Job Processing Flow
1. API validates request and writes job record to PostgreSQL.
2. API pushes job to Redis queue.
3. Worker consumes queue and executes benchmark engine.
4. Result persisted to `benchmark_results` table.
5. Failures retry automatically; max retries routed to DLQ.

## Production Features
- Idempotency key support for safe re-submission
- Retry + DLQ design for distributed workload resilience
- Composite DB index on `gpu_model_id + created_at`
- Structured JSON logging
- Optional OpenTelemetry tracing bootstrap
- NVIDIA runtime adapter + simulated fallback engine

## Repository Structure
- `backend/app/api` - FastAPI routes
- `backend/app/models` - SQLAlchemy models
- `backend/app/schemas` - Pydantic schemas
- `backend/app/services` - Business logic layer
- `backend/app/workers` - Async worker runtime
- `backend/app/database` - DB session management
- `backend/app/core` - Config and shared utilities
- `backend/tests` - Unit/API/DB tests
- `backend/alembic` - Database migrations
- `docker-compose.yml` - Local development stack
- `dockerfile` - Production container image
- `k8s/` - Kubernetes manifests
- `.github/workflows/` - CI/CD pipelines
- `docs/aws_deployment.md` - AWS architecture guide

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Python 3.11+
- PostgreSQL 16+ (or use Docker)
- Redis 7+ (or use Docker)

### Local Development
1. Clone the repository
   ```bash
   git clone https://github.com/<your-username>/gpu-benchmark-platform.git
   cd gpu-benchmark-platform
   ```

2. Copy environment template
   ```bash
   cp .env.example .env
   ```

3. Start services with Docker Compose
   ```bash
   docker compose up --build
   ```

4. Run database migrations
   ```bash
   cd backend
   alembic upgrade head
   ```

5. Access API documentation
   - API: `http://localhost:8000`
   - Swagger UI: `http://localhost:8000/docs`

### Testing
```bash
pip install -r backend/requirements.txt
pytest backend/tests -q
```

## Cloud Deployment
- **Kubernetes**: manifests available in `k8s/`
- **AWS**: reference architecture in `docs/aws_deployment.md` (ALB + ECS/Fargate + RDS + ElastiCache)
- **CI/CD**: GitHub Actions workflows in `.github/workflows/`

## License
MIT License - see LICENSE file for details

## Contributing
Contributions are welcome! Please open an issue or submit a pull request.
