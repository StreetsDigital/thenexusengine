"""
Unified metrics store combining Redis (real-time) and TimescaleDB (historical).

Provides a single interface for the IDR scorer to retrieve bidder metrics
from the appropriate source based on recency and context.
"""

import hashlib
import os
from dataclasses import dataclass
from datetime import datetime
from typing import Any

from src.idr.database.redis_client import (
    DEFAULT_SAMPLE_RATE,
    MockRedisClient,
    RealTimeMetrics,
    RedisMetricsClient,
)
from src.idr.database.timescale_client import (
    BidderPerformance,
    MockTimescaleClient,
    TimescaleClient,
)
from src.idr.models.classified_request import ClassifiedRequest


@dataclass
class BidderMetricsSnapshot:
    """
    Combined metrics snapshot for scoring.

    Merges real-time (Redis) and historical (TimescaleDB) data
    with appropriate weighting based on sample size.
    """

    bidder_code: str
    timestamp: datetime

    # Core rates
    win_rate: float = 0.0
    bid_rate: float = 0.0
    avg_cpm: float = 0.0
    floor_clearance_rate: float = 0.0

    # Latency
    avg_latency_ms: float = 0.0
    p95_latency_ms: float = 0.0

    # Sample info
    total_requests: int = 0
    realtime_requests: int = 0
    historical_requests: int = 0

    # Recency (hours since last activity)
    hours_since_last_bid: float = 0.0

    # Reliability
    timeout_rate: float = 0.0
    error_rate: float = 0.0

    @property
    def sample_size(self) -> int:
        return self.total_requests

    @property
    def has_sufficient_data(self) -> bool:
        """Check if we have enough data for confident scoring."""
        return self.total_requests >= 100

    @property
    def confidence(self) -> float:
        """Confidence score 0-1 based on sample size."""
        if self.total_requests >= 10000:
            return 1.0
        elif self.total_requests >= 1000:
            return 0.9
        elif self.total_requests >= 100:
            return 0.7
        elif self.total_requests >= 10:
            return 0.4
        return 0.1


class MetricsStore:
    """
    Unified interface for bidder performance metrics.

    Combines:
    - Redis: Real-time metrics (last hour)
    - TimescaleDB: Historical aggregates (last 24h+)

    Weighting strategy:
    - Recent data (Redis) weighted higher for responsiveness
    - Historical data provides baseline and fills gaps
    """

    # Weighting factors
    REALTIME_WEIGHT = 0.6  # Weight for real-time (last hour) data
    HISTORICAL_WEIGHT = 0.4  # Weight for historical (24h) data

    def __init__(
        self,
        redis_client: RedisMetricsClient | MockRedisClient | None = None,
        timescale_client: TimescaleClient | MockTimescaleClient | None = None,
    ):
        self.redis = redis_client
        self.timescale = timescale_client

    @classmethod
    def create(
        cls,
        redis_host: str = "localhost",
        redis_port: int = 6379,
        timescale_host: str = "localhost",
        timescale_port: int = 5432,
        timescale_db: str = "idr",
        timescale_user: str = "idr",
        timescale_password: str = "",
        use_mocks: bool = False,
        redis_sample_rate: float | None = None,
        redis_url: str | None = None,
        redis_max_connections: int = 20,
    ) -> "MetricsStore":
        """
        Factory method to create a configured MetricsStore.

        Args:
            redis_sample_rate: Sampling rate for Redis (0.0-1.0). Default from
                              REDIS_SAMPLE_RATE env var or 1.0. Use 0.1 for 10%
                              sampling to reduce Redis commands by 90%.
            redis_url: Redis connection URL (overrides host/port). Falls back to
                      REDIS_URL env var.
            redis_max_connections: Maximum connections in the Redis pool (default: 20).
        """
        # Get sample rate from env var if not specified
        if redis_sample_rate is None:
            redis_sample_rate = float(
                os.getenv("REDIS_SAMPLE_RATE", str(DEFAULT_SAMPLE_RATE))
            )

        # Get Redis URL from env var if not specified
        if redis_url is None:
            redis_url = os.getenv("REDIS_URL")

        if use_mocks:
            return cls(
                redis_client=MockRedisClient(),
                timescale_client=MockTimescaleClient(),
            )

        redis_client = None
        timescale_client = None

        # Try to connect to Redis (prefer URL-based connection for production)
        try:
            redis_client = RedisMetricsClient(
                host=redis_host,
                port=redis_port,
                sample_rate=redis_sample_rate,
                url=redis_url,
                max_connections=redis_max_connections,
            )
            if not redis_client.connect():
                print("Warning: Could not connect to Redis, using mock")
                redis_client = MockRedisClient()
            else:
                pool_info = f"pool={redis_max_connections}" if redis_url else f"{redis_host}:{redis_port}"
                print(f"Redis connected: {pool_info}, sampling={redis_sample_rate:.0%}")
        except ImportError:
            print("Warning: redis package not available, using mock")
            redis_client = MockRedisClient()

        # Try to connect to TimescaleDB
        try:
            timescale_client = TimescaleClient(
                host=timescale_host,
                port=timescale_port,
                database=timescale_db,
                user=timescale_user,
                password=timescale_password,
            )
            if not timescale_client.connect():
                print("Warning: Could not connect to TimescaleDB, using mock")
                timescale_client = MockTimescaleClient()
        except ImportError:
            print("Warning: psycopg2 package not available, using mock")
            timescale_client = MockTimescaleClient()

        return cls(redis_client=redis_client, timescale_client=timescale_client)

    def _context_hash(self, request: ClassifiedRequest) -> str:
        """Generate context hash for request-specific lookups."""
        context = f"{request.country}:{request.device_type}:{request.ad_format}:{request.primary_size}"
        return hashlib.md5(context.encode()).hexdigest()[:12]

    # =========================================================================
    # Recording Events
    # =========================================================================

    def record_request(
        self,
        auction_id: str,
        bidder_code: str,
        request: ClassifiedRequest,
        latency_ms: float,
        had_bid: bool,
        bid_cpm: float | None = None,
        timed_out: bool = False,
        had_error: bool = False,
        floor_price: float | None = None,
    ) -> None:
        """Record a bid request event to both stores."""
        context_hash = self._context_hash(request)

        # Record to Redis (real-time)
        if self.redis:
            self.redis.record_request(
                bidder=bidder_code,
                context_hash=context_hash,
                latency_ms=latency_ms,
                had_bid=had_bid,
                bid_cpm=bid_cpm,
                timed_out=timed_out,
                had_error=had_error,
            )

        # Record to TimescaleDB (historical)
        if self.timescale:
            if floor_price is not None and bid_cpm is not None:
                pass

            self.timescale.record_bid_event(
                auction_id=auction_id,
                bidder_code=bidder_code,
                country=request.country,
                device_type=request.device_type,
                media_type=request.ad_format,
                ad_size=request.primary_size,
                publisher_id=request.publisher_id,
                had_bid=had_bid,
                bid_cpm=bid_cpm,
                latency_ms=latency_ms,
                timed_out=timed_out,
                had_error=had_error,
                floor_price=floor_price,
            )

    def record_win(
        self,
        auction_id: str,
        bidder_code: str,
        request: ClassifiedRequest,
        win_cpm: float,
        clearing_price: float | None = None,
    ) -> None:
        """Record a win event."""
        context_hash = self._context_hash(request)

        # Record to Redis
        if self.redis:
            self.redis.record_win(
                bidder=bidder_code,
                context_hash=context_hash,
                win_cpm=win_cpm,
                clearing_price=clearing_price,
            )

        # Update TimescaleDB event with win
        # Note: In production, you'd update the existing event or use a separate wins table
        if self.timescale:
            self.timescale.record_bid_event(
                auction_id=f"{auction_id}-win",
                bidder_code=bidder_code,
                country=request.country,
                device_type=request.device_type,
                media_type=request.ad_format,
                won=True,
                win_cpm=win_cpm,
            )

    # =========================================================================
    # Reading Metrics
    # =========================================================================

    def get_metrics(
        self,
        bidder_code: str,
        request: ClassifiedRequest | None = None,
    ) -> BidderMetricsSnapshot:
        """
        Get combined metrics for a bidder.

        Args:
            bidder_code: The bidder to get metrics for
            request: Optional request context for context-specific metrics

        Returns:
            BidderMetricsSnapshot with combined real-time and historical data
        """
        context_hash = self._context_hash(request) if request else None

        # Get real-time metrics from Redis
        rt_metrics: RealTimeMetrics | None = None
        rt_p95: float = 0.0
        if self.redis:
            rt_metrics = self.redis.get_metrics(bidder_code, context_hash)
            rt_p95 = self.redis.get_p95_latency(bidder_code)

        # Get historical metrics from TimescaleDB
        hist_metrics: BidderPerformance | None = None
        hist_p95: float = 0.0
        if self.timescale:
            hist_metrics = self.timescale.get_bidder_performance(
                bidder_code=bidder_code,
                hours=24,
                country=request.country if request else None,
                device_type=request.device_type if request else None,
                media_type=request.ad_format if request else None,
            )
            hist_p95 = self.timescale.get_p95_latency(bidder_code, hours=24)

        # Combine metrics
        return self._combine_metrics(
            bidder_code=bidder_code,
            realtime=rt_metrics,
            historical=hist_metrics,
            rt_p95=rt_p95,
            hist_p95=hist_p95,
        )

    def _combine_metrics(
        self,
        bidder_code: str,
        realtime: RealTimeMetrics | None,
        historical: BidderPerformance | None,
        rt_p95: float,
        hist_p95: float,
    ) -> BidderMetricsSnapshot:
        """Combine real-time and historical metrics with weighting."""
        rt_requests = realtime.requests if realtime else 0
        hist_requests = historical.requests if historical else 0
        total_requests = rt_requests + hist_requests

        if total_requests == 0:
            # No data at all
            return BidderMetricsSnapshot(
                bidder_code=bidder_code,
                timestamp=datetime.now(),
            )

        # Calculate effective weights based on available data
        rt_weight = self.REALTIME_WEIGHT if rt_requests > 0 else 0
        hist_weight = self.HISTORICAL_WEIGHT if hist_requests > 0 else 0

        # Normalize weights
        total_weight = rt_weight + hist_weight
        if total_weight > 0:
            rt_weight /= total_weight
            hist_weight /= total_weight

        # Combine rates
        rt_win_rate = realtime.win_rate if realtime else 0
        rt_bid_rate = realtime.bid_rate if realtime else 0
        rt_avg_cpm = realtime.avg_bid_cpm if realtime else 0
        rt_timeout_rate = realtime.timeout_rate if realtime else 0
        rt_error_rate = realtime.error_rate if realtime else 0
        rt_avg_latency = realtime.avg_latency_ms if realtime else 0

        hist_win_rate = historical.win_rate if historical else 0
        hist_bid_rate = historical.bid_rate if historical else 0
        hist_avg_cpm = historical.avg_bid_cpm if historical else 0
        hist_floor_clearance = historical.floor_clearance_rate if historical else 0
        hist_avg_latency = historical.avg_latency_ms if historical else 0

        return BidderMetricsSnapshot(
            bidder_code=bidder_code,
            timestamp=datetime.now(),
            win_rate=rt_win_rate * rt_weight + hist_win_rate * hist_weight,
            bid_rate=rt_bid_rate * rt_weight + hist_bid_rate * hist_weight,
            avg_cpm=rt_avg_cpm * rt_weight + hist_avg_cpm * hist_weight,
            floor_clearance_rate=hist_floor_clearance,  # Only from historical
            avg_latency_ms=rt_avg_latency * rt_weight + hist_avg_latency * hist_weight,
            p95_latency_ms=rt_p95 * rt_weight + hist_p95 * hist_weight,
            total_requests=total_requests,
            realtime_requests=rt_requests,
            historical_requests=hist_requests,
            timeout_rate=rt_timeout_rate * rt_weight,  # Recent timeouts more relevant
            error_rate=rt_error_rate * rt_weight,
        )

    def get_all_metrics(self) -> dict[str, BidderMetricsSnapshot]:
        """Get metrics for all known bidders."""
        bidders: set[str] = set()

        # Collect bidders from Redis
        if self.redis:
            rt_metrics = self.redis.get_all_bidder_metrics()
            bidders.update(rt_metrics.keys())

        # Collect bidders from TimescaleDB
        if self.timescale:
            hist_stats = self.timescale.get_all_bidder_stats(hours=24)
            bidders.update(bp.bidder_code for bp in hist_stats)

        # Get combined metrics for each
        return {bidder: self.get_metrics(bidder) for bidder in bidders}

    # =========================================================================
    # Utility
    # =========================================================================

    def health_check(self) -> dict[str, Any]:
        """Check health of both stores."""
        status = {
            "redis": {"status": "unavailable"},
            "timescale": {"status": "unavailable"},
        }

        if self.redis:
            try:
                if hasattr(self.redis, "get_stats"):
                    status["redis"] = {"status": "connected", **self.redis.get_stats()}
                else:
                    status["redis"] = {"status": "connected (mock)"}
            except Exception as e:
                status["redis"] = {"status": "error", "error": str(e)}

        if self.timescale:
            try:
                if self.timescale.is_connected:
                    status["timescale"] = {"status": "connected"}
                else:
                    status["timescale"] = {"status": "disconnected"}
            except Exception as e:
                status["timescale"] = {"status": "error", "error": str(e)}

        return status
