"""
db_tuning.py — Query plan analysis for the benchmark_results table.

Usage (run directly against a live database):
    DATABASE_URL=postgresql+psycopg2://user:pass@host/db python db_tuning.py

The script runs EXPLAIN ANALYZE on the hot query path:
  - Filter by gpu_model (via JOIN to gpu_models)
  - Sort by benchmark_results.created_at DESC

Why a composite index on (gpu_model, created_at DESC) prevents a Seq Scan
--------------------------------------------------------------------------
PostgreSQL must satisfy two predicates for this query:
  1. Equality filter on gpu_model (high cardinality key lookup)
  2. ORDER BY created_at DESC (requires sorted access)

Without the composite index the planner must:
  a. Seq Scan the entire benchmark_results table (O(n)).
  b. Filter rows in memory for the matching gpu_model.
  c. Sort the surviving rows by created_at (O(m log m)).

With a B-tree composite index on (gpu_model, created_at DESC):
  a. The planner performs an Index Scan on the leading column (gpu_model),
     jumping directly to the relevant leaf pages — O(log n + k) where k is
     the result set size.
  b. Because the index is already ordered by created_at DESC, no explicit
     Sort node is needed; PostgreSQL reads the index entries in index order.
  c. For small k (typical time-series look-ups) this is dramatically cheaper
     than a Seq Scan + Sort, and the planner will always prefer it once the
     statistics show low selectivity for gpu_model.
"""

import logging
import os
import sys
from typing import Any

from sqlalchemy import create_engine, text

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s  %(levelname)-8s  %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%S",
)
log = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Database URL — override via environment variable for production use.
# ---------------------------------------------------------------------------
DATABASE_URL: str = os.getenv(
    "DATABASE_URL",
    "postgresql+psycopg2://postgres:postgres@localhost:5432/benchmark_db",
)

# ---------------------------------------------------------------------------
# The query whose plan we want to analyse.
# We JOIN through benchmarks -> gpu_models so we can filter by the human-
# readable gpu model name, which is what the API surface exposes.
# ---------------------------------------------------------------------------
EXPLAIN_QUERY = text(
    """
    EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
    SELECT
        br.id,
        br.throughput,
        br.latency_ms,
        br.memory_bandwidth_gbps,
        br.raw_metrics,
        br.created_at
    FROM
        benchmark_results AS br
        JOIN benchmarks   AS b  ON b.id  = br.benchmark_id
        JOIN gpu_models   AS gm ON gm.id = b.gpu_model_id
    WHERE
        gm.model_name = :gpu_model
    ORDER BY
        br.created_at DESC
    LIMIT :limit_val
    """
)


def run_explain_analyze(gpu_model: str, limit: int = 100) -> list[str]:
    """Execute EXPLAIN ANALYZE and return the plan lines.

    Args:
        gpu_model: The GPU model name to filter by (e.g. ``"H100"``).
        limit:     Maximum rows to fetch; defaults to 100.

    Returns:
        A list of strings, one per line of the PostgreSQL query plan.

    Raises:
        sqlalchemy.exc.OperationalError: If the database is unreachable.
    """
    engine = create_engine(DATABASE_URL, echo=False, future=True)

    log.info("Connecting to database: %s", DATABASE_URL.split("@")[-1])
    log.info("Running EXPLAIN ANALYZE for gpu_model=%r, limit=%d", gpu_model, limit)

    with engine.connect() as conn:
        result = conn.execute(EXPLAIN_QUERY, {"gpu_model": gpu_model, "limit_val": limit})
        plan_lines: list[str] = [row[0] for row in result]

    engine.dispose()
    return plan_lines


def log_query_plan(plan_lines: list[Any]) -> None:
    """Write the query execution plan to the logger."""
    log.info("=" * 72)
    log.info("QUERY EXECUTION PLAN")
    log.info("=" * 72)
    for line in plan_lines:
        log.info(line)
    log.info("=" * 72)

    # Highlight whether a Seq Scan is present — useful as a quick regression
    # check in a CI / DBA workflow.
    full_plan = "\n".join(plan_lines)
    if "Seq Scan on benchmark_results" in full_plan:
        log.warning(
            "Seq Scan detected on benchmark_results! "
            "Ensure the composite index on (gpu_model_id, created_at DESC) "
            "exists and that table statistics are up to date (ANALYZE)."
        )
    else:
        log.info("No Seq Scan on benchmark_results — index is being used correctly.")


if __name__ == "__main__":
    gpu_model_arg = sys.argv[1] if len(sys.argv) > 1 else "H100"
    plan = run_explain_analyze(gpu_model=gpu_model_arg)
    log_query_plan(plan)
