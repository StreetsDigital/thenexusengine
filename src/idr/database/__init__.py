"""
IDR Database Package - Redis + TimescaleDB for performance metrics.

Redis: Real-time metrics (last hour) for fast auction-time lookups
TimescaleDB: Historical data for analytics and ML training
"""

from src.idr.database.event_pipeline import (
    AuctionEvent,
    EventPipeline,
    EventType,
    SyncEventPipeline,
)
from src.idr.database.metrics_store import BidderMetricsSnapshot, MetricsStore
from src.idr.database.redis_client import (
    MockRedisClient,
    RealTimeMetrics,
    RedisMetricsClient,
)
from src.idr.database.timescale_client import (
    BidderPerformance,
    MockTimescaleClient,
    TimescaleClient,
)

__all__ = [
    "RedisMetricsClient",
    "RealTimeMetrics",
    "MockRedisClient",
    "TimescaleClient",
    "BidderPerformance",
    "MockTimescaleClient",
    "MetricsStore",
    "BidderMetricsSnapshot",
    "EventPipeline",
    "SyncEventPipeline",
    "AuctionEvent",
    "EventType",
]
