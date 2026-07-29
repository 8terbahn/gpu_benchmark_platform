from fastapi import APIRouter

from app.core.config import settings

router = APIRouter(tags=['health'])


@router.get('/')
async def health_check() -> dict[str, str]:
    return {
        'status': 'healthy',
        'service': 'gpu_benchmark_engine',
        'environment': settings.environment,
    }

