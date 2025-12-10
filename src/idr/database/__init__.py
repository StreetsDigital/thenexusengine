"""
IDR Database Package - Redis + TimescaleDB for performance metrics.

Redis: Real-time metrics (last hour) for fast auction-time lookups
TimescaleDB: Historical data for analytics and ML training
"""

from src.idr.database.redis_client import RedisMetricsClient, RealTimeMetrics, MockRedisClient
from src.idr.database.timescale_client import TimescaleClient, BidderPerformance, MockTimescaleClient
from src.idr.database.metrics_store import MetricsStore, BidderMetricsSnapshot
from src.idr.database.event_pipeline import EventPipeline, SyncEventPipeline, AuctionEvent, EventType

__all__ = [
    'RedisMetricsClient',
    'RealTimeMetrics',
    'MockRedisClient',
    'TimescaleClient',
    'BidderPerformance',
    'MockTimescaleClient',
    'MetricsStore',
    'BidderMetricsSnapshot',
    'EventPipeline',
    'SyncEventPipeline',
    'AuctionEvent',
    'EventType',
]
