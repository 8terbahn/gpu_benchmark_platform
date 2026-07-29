import json
from typing import Any

from redis.asyncio import Redis

from app.core.config import settings


class RedisQueueService:
    def __init__(self, redis_url: str, queue_name: str, dlq_name: str) -> None:
        self._redis = Redis.from_url(redis_url, decode_responses=True)
        self._queue_name = queue_name
        self._dlq_name = dlq_name

    async def enqueue(self, payload: dict[str, Any]) -> None:
        await self._redis.lpush(self._queue_name, json.dumps(payload))

    async def enqueue_retry(self, payload: dict[str, Any]) -> None:
        payload["attempt"] = int(payload.get("attempt", 0)) + 1
        await self._redis.lpush(self._queue_name, json.dumps(payload))

    async def dequeue(self, timeout: int = 5) -> dict[str, Any] | None:
        item = await self._redis.brpop(self._queue_name, timeout=timeout)
        if not item:
            return None
        return json.loads(item[1])

    async def move_to_dlq(self, payload: dict[str, Any], reason: str) -> None:
        payload["dlq_reason"] = reason
        await self._redis.lpush(self._dlq_name, json.dumps(payload))

    async def close(self) -> None:
        await self._redis.aclose()


queue_service = RedisQueueService(settings.redis_url, settings.redis_queue_name, settings.redis_dlq_name)
