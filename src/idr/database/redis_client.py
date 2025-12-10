"""
Redis client for real-time bidder metrics.

Stores hot metrics with automatic expiry for fast lookups during auctions.
Uses Redis hashes and sorted sets for efficient aggregation.
"""

import json
import time
from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import Any, Optional

try:
    import redis
    REDIS_AVAILABLE = True
except ImportError:
    REDIS_AVAILABLE = False


@dataclass
class RealTimeMetrics:
    """Real-time metrics for a bidder in a specific context."""
    bidder_code: str

    # Counts (last hour)
    requests: int = 0
    bids: int = 0
    wins: int = 0
    timeouts: int = 0
    errors: int = 0

    # Aggregates
    total_bid_value: float = 0.0
    total_win_value: float = 0.0
    total_latency_ms: float = 0.0

    # Computed
    @property
    def bid_rate(self) -> float:
        return self.bids / self.requests if self.requests > 0 else 0.0

    @property
    def win_rate(self) -> float:
        return self.wins / self.bids if self.bids > 0 else 0.0

    @property
    def avg_bid_cpm(self) -> float:
        return self.total_bid_value / self.bids if self.bids > 0 else 0.0

    @property
    def avg_win_cpm(self) -> float:
        return self.total_win_value / self.wins if self.wins > 0 else 0.0

    @property
    def avg_latency_ms(self) -> float:
        return self.total_latency_ms / self.requests if self.requests > 0 else 0.0

    @property
    def timeout_rate(self) -> float:
        return self.timeouts / self.requests if self.requests > 0 else 0.0

    @property
    def error_rate(self) -> float:
        return self.errors / self.requests if self.requests > 0 else 0.0


class RedisMetricsClient:
    """
    Redis client for real-time bidder performance metrics.

    Key structure:
    - idr:metrics:{bidder}:{context_hash} -> Hash of metrics
    - idr:latency:{bidder} -> Sorted set of recent latencies
    - idr:bids:{bidder} -> Sorted set of recent bid values
    - idr:requests:count -> HyperLogLog for unique request counting
    """

    # Key prefixes
    PREFIX = "idr"
    METRICS_KEY = f"{PREFIX}:metrics"
    LATENCY_KEY = f"{PREFIX}:latency"
    BIDS_KEY = f"{PREFIX}:bids"
    WINS_KEY = f"{PREFIX}:wins"
    GLOBAL_KEY = f"{PREFIX}:global"

    # TTLs
    METRICS_TTL = 3600  # 1 hour
    LATENCY_WINDOW = 300  # 5 minutes for P95 calculation

    def __init__(
        self,
        host: str = "localhost",
        port: int = 6379,
        db: int = 0,
        password: Optional[str] = None,
        decode_responses: bool = True,
    ):
        if not REDIS_AVAILABLE:
            raise ImportError("redis package not installed. Run: pip install redis")

        self.client = redis.Redis(
            host=host,
            port=port,
            db=db,
            password=password,
            decode_responses=decode_responses,
        )
        self._connected = False

    def connect(self) -> bool:
        """Test connection to Redis."""
        try:
            self.client.ping()
            self._connected = True
            return True
        except redis.ConnectionError:
            self._connected = False
            return False

    @property
    def is_connected(self) -> bool:
        return self._connected

    def _context_key(self, bidder: str, context_hash: str) -> str:
        """Build key for bidder+context metrics."""
        return f"{self.METRICS_KEY}:{bidder}:{context_hash}"

    def _global_key(self, bidder: str) -> str:
        """Build key for global bidder metrics."""
        return f"{self.GLOBAL_KEY}:{bidder}"

    # =========================================================================
    # Recording Events
    # =========================================================================

    def record_request(
        self,
        bidder: str,
        context_hash: str,
        latency_ms: float,
        had_bid: bool,
        bid_cpm: Optional[float] = None,
        timed_out: bool = False,
        had_error: bool = False,
    ) -> None:
        """
        Record a bid request event.

        Args:
            bidder: Bidder code
            context_hash: Hash of request context (geo, device, format, etc.)
            latency_ms: Response latency in milliseconds
            had_bid: Whether bidder returned a bid
            bid_cpm: CPM of the bid (if any)
            timed_out: Whether request timed out
            had_error: Whether there was an error
        """
        now = time.time()
        pipe = self.client.pipeline()

        # Update context-specific metrics
        ctx_key = self._context_key(bidder, context_hash)
        pipe.hincrby(ctx_key, "requests", 1)
        pipe.hincrbyfloat(ctx_key, "total_latency_ms", latency_ms)

        if had_bid:
            pipe.hincrby(ctx_key, "bids", 1)
            if bid_cpm:
                pipe.hincrbyfloat(ctx_key, "total_bid_value", bid_cpm)

        if timed_out:
            pipe.hincrby(ctx_key, "timeouts", 1)

        if had_error:
            pipe.hincrby(ctx_key, "errors", 1)

        pipe.expire(ctx_key, self.METRICS_TTL)

        # Update global metrics
        global_key = self._global_key(bidder)
        pipe.hincrby(global_key, "requests", 1)
        pipe.hincrbyfloat(global_key, "total_latency_ms", latency_ms)

        if had_bid:
            pipe.hincrby(global_key, "bids", 1)
            if bid_cpm:
                pipe.hincrbyfloat(global_key, "total_bid_value", bid_cpm)

        if timed_out:
            pipe.hincrby(global_key, "timeouts", 1)

        if had_error:
            pipe.hincrby(global_key, "errors", 1)

        # Record latency in sorted set for P95 calculation
        latency_key = f"{self.LATENCY_KEY}:{bidder}"
        pipe.zadd(latency_key, {f"{now}:{latency_ms}": now})
        pipe.zremrangebyscore(latency_key, "-inf", now - self.LATENCY_WINDOW)

        # Record bid value if present
        if had_bid and bid_cpm:
            bids_key = f"{self.BIDS_KEY}:{bidder}"
            pipe.zadd(bids_key, {f"{now}:{bid_cpm}": now})
            pipe.zremrangebyscore(bids_key, "-inf", now - self.LATENCY_WINDOW)

        pipe.execute()

    def record_win(
        self,
        bidder: str,
        context_hash: str,
        win_cpm: float,
        clearing_price: Optional[float] = None,
    ) -> None:
        """
        Record a win event.

        Args:
            bidder: Bidder code
            context_hash: Hash of request context
            win_cpm: Winning bid CPM
            clearing_price: Second price (if available)
        """
        now = time.time()
        pipe = self.client.pipeline()

        # Update context-specific wins
        ctx_key = self._context_key(bidder, context_hash)
        pipe.hincrby(ctx_key, "wins", 1)
        pipe.hincrbyfloat(ctx_key, "total_win_value", win_cpm)
        pipe.expire(ctx_key, self.METRICS_TTL)

        # Update global wins
        global_key = self._global_key(bidder)
        pipe.hincrby(global_key, "wins", 1)
        pipe.hincrbyfloat(global_key, "total_win_value", win_cpm)

        # Record win in sorted set
        wins_key = f"{self.WINS_KEY}:{bidder}"
        pipe.zadd(wins_key, {f"{now}:{win_cpm}": now})
        pipe.zremrangebyscore(wins_key, "-inf", now - self.LATENCY_WINDOW)

        pipe.execute()

    # =========================================================================
    # Reading Metrics
    # =========================================================================

    def get_metrics(self, bidder: str, context_hash: Optional[str] = None) -> RealTimeMetrics:
        """
        Get metrics for a bidder.

        Args:
            bidder: Bidder code
            context_hash: Optional context hash for context-specific metrics.
                         If None, returns global metrics.

        Returns:
            RealTimeMetrics object
        """
        if context_hash:
            key = self._context_key(bidder, context_hash)
        else:
            key = self._global_key(bidder)

        data = self.client.hgetall(key)

        return RealTimeMetrics(
            bidder_code=bidder,
            requests=int(data.get("requests", 0)),
            bids=int(data.get("bids", 0)),
            wins=int(data.get("wins", 0)),
            timeouts=int(data.get("timeouts", 0)),
            errors=int(data.get("errors", 0)),
            total_bid_value=float(data.get("total_bid_value", 0.0)),
            total_win_value=float(data.get("total_win_value", 0.0)),
            total_latency_ms=float(data.get("total_latency_ms", 0.0)),
        )

    def get_p95_latency(self, bidder: str) -> float:
        """Get P95 latency for a bidder over the last 5 minutes."""
        latency_key = f"{self.LATENCY_KEY}:{bidder}"

        # Get all latencies in window
        entries = self.client.zrange(latency_key, 0, -1)
        if not entries:
            return 0.0

        # Parse latencies from entries (format: "timestamp:latency")
        latencies = []
        for entry in entries:
            try:
                _, latency_str = entry.rsplit(":", 1)
                latencies.append(float(latency_str))
            except (ValueError, AttributeError):
                continue

        if not latencies:
            return 0.0

        # Calculate P95
        latencies.sort()
        p95_index = int(len(latencies) * 0.95)
        return latencies[min(p95_index, len(latencies) - 1)]

    def get_recent_bid_stats(self, bidder: str) -> dict[str, float]:
        """Get recent bid statistics for a bidder."""
        bids_key = f"{self.BIDS_KEY}:{bidder}"

        entries = self.client.zrange(bids_key, 0, -1)
        if not entries:
            return {"count": 0, "avg_cpm": 0.0, "max_cpm": 0.0, "min_cpm": 0.0}

        cpms = []
        for entry in entries:
            try:
                _, cpm_str = entry.rsplit(":", 1)
                cpms.append(float(cpm_str))
            except (ValueError, AttributeError):
                continue

        if not cpms:
            return {"count": 0, "avg_cpm": 0.0, "max_cpm": 0.0, "min_cpm": 0.0}

        return {
            "count": len(cpms),
            "avg_cpm": sum(cpms) / len(cpms),
            "max_cpm": max(cpms),
            "min_cpm": min(cpms),
        }

    def get_all_bidder_metrics(self) -> dict[str, RealTimeMetrics]:
        """Get global metrics for all bidders."""
        # Find all global keys
        pattern = f"{self.GLOBAL_KEY}:*"
        keys = list(self.client.scan_iter(match=pattern))

        results = {}
        for key in keys:
            bidder = key.split(":")[-1]
            results[bidder] = self.get_metrics(bidder)

        return results

    # =========================================================================
    # Utility
    # =========================================================================

    def flush_bidder(self, bidder: str) -> None:
        """Clear all metrics for a bidder."""
        patterns = [
            f"{self.METRICS_KEY}:{bidder}:*",
            f"{self.GLOBAL_KEY}:{bidder}",
            f"{self.LATENCY_KEY}:{bidder}",
            f"{self.BIDS_KEY}:{bidder}",
            f"{self.WINS_KEY}:{bidder}",
        ]

        for pattern in patterns:
            for key in self.client.scan_iter(match=pattern):
                self.client.delete(key)

    def get_stats(self) -> dict[str, Any]:
        """Get Redis stats for monitoring."""
        info = self.client.info()
        return {
            "connected_clients": info.get("connected_clients", 0),
            "used_memory_human": info.get("used_memory_human", "0B"),
            "total_keys": self.client.dbsize(),
        }


class MockRedisClient:
    """In-memory mock for testing without Redis."""

    def __init__(self):
        self._data: dict[str, dict] = {}
        self._sorted_sets: dict[str, list] = {}
        self._connected = True

    def connect(self) -> bool:
        return True

    @property
    def is_connected(self) -> bool:
        return self._connected

    def record_request(self, bidder: str, context_hash: str, **kwargs) -> None:
        key = f"global:{bidder}"
        if key not in self._data:
            self._data[key] = {"requests": 0, "bids": 0, "wins": 0, "timeouts": 0,
                              "errors": 0, "total_bid_value": 0.0, "total_win_value": 0.0,
                              "total_latency_ms": 0.0}

        self._data[key]["requests"] += 1
        self._data[key]["total_latency_ms"] += kwargs.get("latency_ms", 0)

        if kwargs.get("had_bid"):
            self._data[key]["bids"] += 1
            self._data[key]["total_bid_value"] += kwargs.get("bid_cpm", 0) or 0

        if kwargs.get("timed_out"):
            self._data[key]["timeouts"] += 1

        if kwargs.get("had_error"):
            self._data[key]["errors"] += 1

    def record_win(self, bidder: str, context_hash: str, win_cpm: float, **kwargs) -> None:
        key = f"global:{bidder}"
        if key not in self._data:
            self._data[key] = {"requests": 0, "bids": 0, "wins": 0, "timeouts": 0,
                              "errors": 0, "total_bid_value": 0.0, "total_win_value": 0.0,
                              "total_latency_ms": 0.0}

        self._data[key]["wins"] += 1
        self._data[key]["total_win_value"] += win_cpm

    def get_metrics(self, bidder: str, context_hash: Optional[str] = None) -> RealTimeMetrics:
        key = f"global:{bidder}"
        data = self._data.get(key, {})

        return RealTimeMetrics(
            bidder_code=bidder,
            requests=data.get("requests", 0),
            bids=data.get("bids", 0),
            wins=data.get("wins", 0),
            timeouts=data.get("timeouts", 0),
            errors=data.get("errors", 0),
            total_bid_value=data.get("total_bid_value", 0.0),
            total_win_value=data.get("total_win_value", 0.0),
            total_latency_ms=data.get("total_latency_ms", 0.0),
        )

    def get_p95_latency(self, bidder: str) -> float:
        return 150.0  # Mock value

    def get_all_bidder_metrics(self) -> dict[str, RealTimeMetrics]:
        results = {}
        for key in self._data:
            if key.startswith("global:"):
                bidder = key.split(":", 1)[1]
                results[bidder] = self.get_metrics(bidder)
        return results

    def flush_bidder(self, bidder: str) -> None:
        key = f"global:{bidder}"
        if key in self._data:
            del self._data[key]
