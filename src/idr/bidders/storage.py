"""
Redis-based storage for OpenRTB bidder configurations.

Bidder configurations are stored in Redis for fast access by both
the Python IDR service and the Go PBS server.

Redis Key Structure:
    nexus:bidders (hash)           - bidder_code -> JSON config
    nexus:bidders:active (set)     - Set of active bidder codes
    nexus:bidders:index (sorted set) - bidder_code -> priority score
    nexus:bidders:stats:{code} (hash) - Real-time stats for bidder
"""

import os
from datetime import datetime

from .models import BidderConfig, BidderStatus

try:
    import redis

    REDIS_AVAILABLE = True
except ImportError:
    REDIS_AVAILABLE = False


# Redis key patterns
REDIS_BIDDERS_HASH = "nexus:bidders"
REDIS_BIDDERS_ACTIVE = "nexus:bidders:active"
REDIS_BIDDERS_INDEX = "nexus:bidders:index"
REDIS_BIDDERS_STATS_PREFIX = "nexus:bidders:stats:"


class BidderStorage:
    """
    Redis storage for bidder configurations.

    This class handles persistence of bidder configurations to Redis,
    providing fast access for both Python and Go services.
    """

    def __init__(self, redis_url: str = "redis://localhost:6379"):
        """
        Initialize bidder storage.

        Args:
            redis_url: Redis connection URL
        """
        self._redis: redis.Redis | None = None
        self._redis_url = redis_url

        if REDIS_AVAILABLE:
            self._connect(redis_url)

    def _connect(self, redis_url: str) -> bool:
        """Connect to Redis."""
        try:
            self._redis = redis.from_url(redis_url, decode_responses=True)
            self._redis.ping()
            return True
        except Exception as e:
            print(f"Redis connection failed: {e}")
            self._redis = None
            return False

    @property
    def is_connected(self) -> bool:
        """Check if Redis is connected."""
        if self._redis is None:
            return False
        try:
            self._redis.ping()
            return True
        except Exception:
            return False

    def save(self, config: BidderConfig) -> bool:
        """
        Save a bidder configuration to Redis.

        Args:
            config: The bidder configuration to save

        Returns:
            True if saved successfully
        """
        if not self._redis:
            return False

        try:
            # Update timestamp
            config.updated_at = datetime.utcnow().isoformat()

            # Use pipeline for atomic operations
            pipe = self._redis.pipeline()

            # Store config in hash
            pipe.hset(REDIS_BIDDERS_HASH, config.bidder_code, config.to_json())

            # Update active set
            if config.is_enabled:
                pipe.sadd(REDIS_BIDDERS_ACTIVE, config.bidder_code)
            else:
                pipe.srem(REDIS_BIDDERS_ACTIVE, config.bidder_code)

            # Update priority index
            pipe.zadd(REDIS_BIDDERS_INDEX, {config.bidder_code: config.priority})

            pipe.execute()
            return True

        except Exception as e:
            print(f"Failed to save bidder config: {e}")
            return False

    def get(self, bidder_code: str) -> BidderConfig | None:
        """
        Get a bidder configuration by code.

        Args:
            bidder_code: The unique bidder identifier

        Returns:
            BidderConfig if found, None otherwise
        """
        if not self._redis:
            return None

        try:
            json_str = self._redis.hget(REDIS_BIDDERS_HASH, bidder_code)
            if json_str:
                return BidderConfig.from_json(json_str)
            return None
        except Exception as e:
            print(f"Failed to get bidder config: {e}")
            return None

    def get_all(self) -> dict[str, BidderConfig]:
        """
        Get all bidder configurations.

        Returns:
            Dictionary mapping bidder_code to BidderConfig
        """
        if not self._redis:
            return {}

        try:
            all_configs = self._redis.hgetall(REDIS_BIDDERS_HASH)
            result = {}
            for code, json_str in all_configs.items():
                try:
                    result[code] = BidderConfig.from_json(json_str)
                except Exception:
                    continue
            return result
        except Exception as e:
            print(f"Failed to get all bidder configs: {e}")
            return {}

    def get_active(self) -> list[BidderConfig]:
        """
        Get all active (enabled) bidder configurations.

        Returns:
            List of active BidderConfig objects, sorted by priority
        """
        if not self._redis:
            return []

        try:
            # Get active bidder codes
            active_codes = self._redis.smembers(REDIS_BIDDERS_ACTIVE)

            if not active_codes:
                return []

            # Get configs for active bidders
            configs = []
            for code in active_codes:
                config = self.get(code)
                if config and config.is_enabled:
                    configs.append(config)

            # Sort by priority (descending)
            configs.sort(key=lambda c: c.priority, reverse=True)
            return configs

        except Exception as e:
            print(f"Failed to get active bidders: {e}")
            return []

    def get_by_priority(self, limit: int = 100) -> list[BidderConfig]:
        """
        Get bidders sorted by priority.

        Args:
            limit: Maximum number of bidders to return

        Returns:
            List of BidderConfig sorted by priority (descending)
        """
        if not self._redis:
            return []

        try:
            # Get sorted bidder codes
            codes = self._redis.zrevrange(REDIS_BIDDERS_INDEX, 0, limit - 1)

            configs = []
            for code in codes:
                config = self.get(code)
                if config:
                    configs.append(config)
            return configs

        except Exception as e:
            print(f"Failed to get bidders by priority: {e}")
            return []

    def delete(self, bidder_code: str) -> bool:
        """
        Delete a bidder configuration.

        Args:
            bidder_code: The bidder identifier to delete

        Returns:
            True if deleted successfully
        """
        if not self._redis:
            return False

        try:
            pipe = self._redis.pipeline()
            pipe.hdel(REDIS_BIDDERS_HASH, bidder_code)
            pipe.srem(REDIS_BIDDERS_ACTIVE, bidder_code)
            pipe.zrem(REDIS_BIDDERS_INDEX, bidder_code)
            pipe.delete(f"{REDIS_BIDDERS_STATS_PREFIX}{bidder_code}")
            pipe.execute()
            return True
        except Exception as e:
            print(f"Failed to delete bidder: {e}")
            return False

    def exists(self, bidder_code: str) -> bool:
        """Check if a bidder configuration exists."""
        if not self._redis:
            return False
        try:
            return self._redis.hexists(REDIS_BIDDERS_HASH, bidder_code)
        except Exception:
            return False

    def list_codes(self) -> list[str]:
        """Get all bidder codes."""
        if not self._redis:
            return []
        try:
            return list(self._redis.hkeys(REDIS_BIDDERS_HASH))
        except Exception:
            return []

    def list_active_codes(self) -> list[str]:
        """Get active bidder codes."""
        if not self._redis:
            return []
        try:
            return list(self._redis.smembers(REDIS_BIDDERS_ACTIVE))
        except Exception:
            return []

    def set_status(self, bidder_code: str, status: BidderStatus) -> bool:
        """
        Update a bidder's status.

        Args:
            bidder_code: The bidder identifier
            status: New status

        Returns:
            True if updated successfully
        """
        config = self.get(bidder_code)
        if not config:
            return False

        config.status = status
        return self.save(config)

    def update_stats(
        self,
        bidder_code: str,
        requests: int = 0,
        bids: int = 0,
        wins: int = 0,
        errors: int = 0,
        latency_ms: float = 0.0,
        bid_cpm: float = 0.0,
    ) -> bool:
        """
        Update bidder statistics incrementally.

        Uses Redis HINCRBY for atomic updates.

        Args:
            bidder_code: The bidder identifier
            requests: Number of requests to add
            bids: Number of bids to add
            wins: Number of wins to add
            errors: Number of errors to add
            latency_ms: Latency to track (for averaging)
            bid_cpm: Bid CPM to track (for averaging)

        Returns:
            True if updated successfully
        """
        if not self._redis:
            return False

        try:
            stats_key = f"{REDIS_BIDDERS_STATS_PREFIX}{bidder_code}"
            pipe = self._redis.pipeline()

            if requests > 0:
                pipe.hincrby(stats_key, "total_requests", requests)
            if bids > 0:
                pipe.hincrby(stats_key, "total_bids", bids)
            if wins > 0:
                pipe.hincrby(stats_key, "total_wins", wins)
            if errors > 0:
                pipe.hincrby(stats_key, "total_errors", errors)

            # For averages, store sum and count
            if latency_ms > 0:
                pipe.hincrbyfloat(stats_key, "latency_sum", latency_ms)
                pipe.hincrby(stats_key, "latency_count", 1)
            if bid_cpm > 0:
                pipe.hincrbyfloat(stats_key, "cpm_sum", bid_cpm)
                pipe.hincrby(stats_key, "cpm_count", 1)

            # Update last active timestamp
            pipe.hset(stats_key, "last_active_at", datetime.utcnow().isoformat())

            pipe.execute()
            return True

        except Exception as e:
            print(f"Failed to update bidder stats: {e}")
            return False

    def get_stats(self, bidder_code: str) -> dict:
        """
        Get real-time statistics for a bidder.

        Returns:
            Dictionary with bidder statistics
        """
        if not self._redis:
            return {}

        try:
            stats_key = f"{REDIS_BIDDERS_STATS_PREFIX}{bidder_code}"
            stats = self._redis.hgetall(stats_key)

            if not stats:
                return {}

            total_requests = int(stats.get("total_requests", 0))
            total_bids = int(stats.get("total_bids", 0))
            total_wins = int(stats.get("total_wins", 0))
            total_errors = int(stats.get("total_errors", 0))
            latency_sum = float(stats.get("latency_sum", 0))
            latency_count = int(stats.get("latency_count", 0))
            cpm_sum = float(stats.get("cpm_sum", 0))
            cpm_count = int(stats.get("cpm_count", 0))

            return {
                "total_requests": total_requests,
                "total_bids": total_bids,
                "total_wins": total_wins,
                "total_errors": total_errors,
                "bid_rate": total_bids / total_requests if total_requests > 0 else 0,
                "win_rate": total_wins / total_bids if total_bids > 0 else 0,
                "error_rate": total_errors / total_requests
                if total_requests > 0
                else 0,
                "avg_latency_ms": latency_sum / latency_count
                if latency_count > 0
                else 0,
                "avg_bid_cpm": cpm_sum / cpm_count if cpm_count > 0 else 0,
                "last_active_at": stats.get("last_active_at"),
            }

        except Exception as e:
            print(f"Failed to get bidder stats: {e}")
            return {}

    def sync_stats_to_config(self, bidder_code: str) -> bool:
        """
        Sync real-time stats to the bidder config.

        Call this periodically to update the stored config
        with latest statistics.
        """
        stats = self.get_stats(bidder_code)
        if not stats:
            return False

        config = self.get(bidder_code)
        if not config:
            return False

        config.total_requests = stats.get("total_requests", 0)
        config.total_bids = stats.get("total_bids", 0)
        config.total_wins = stats.get("total_wins", 0)
        config.total_errors = stats.get("total_errors", 0)
        config.avg_latency_ms = stats.get("avg_latency_ms", 0)
        config.avg_bid_cpm = stats.get("avg_bid_cpm", 0)
        config.last_active_at = stats.get("last_active_at")

        return self.save(config)

    def reset_stats(self, bidder_code: str) -> bool:
        """Reset statistics for a bidder."""
        if not self._redis:
            return False
        try:
            self._redis.delete(f"{REDIS_BIDDERS_STATS_PREFIX}{bidder_code}")
            return True
        except Exception:
            return False

    def get_for_publisher(
        self,
        publisher_id: str,
        country: str | None = None,
    ) -> list[BidderConfig]:
        """
        Get bidders available for a specific publisher.

        Filters based on allowed/blocked publishers and countries.

        Args:
            publisher_id: The publisher identifier
            country: Optional country code for geo filtering

        Returns:
            List of available BidderConfig objects
        """
        active = self.get_active()
        result = []

        for config in active:
            # Check publisher restrictions
            if config.blocked_publishers and publisher_id in config.blocked_publishers:
                continue
            if (
                config.allowed_publishers
                and publisher_id not in config.allowed_publishers
            ):
                continue

            # Check geo restrictions
            if country:
                if config.blocked_countries and country in config.blocked_countries:
                    continue
                if config.allowed_countries and country not in config.allowed_countries:
                    continue

            result.append(config)

        return result


# Global storage instance
_bidder_storage: BidderStorage | None = None


def get_bidder_storage(redis_url: str | None = None) -> BidderStorage:
    """Get the global bidder storage instance."""
    global _bidder_storage

    if _bidder_storage is None:
        url = redis_url or os.environ.get("REDIS_URL", "redis://localhost:6379")
        _bidder_storage = BidderStorage(redis_url=url)

    return _bidder_storage
