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

# Publisher-specific bidder keys
REDIS_PUB_BIDDERS_PREFIX = "nexus:publishers:"  # {prefix}{pub_id}:bidders (hash)
REDIS_PUB_BIDDERS_ENABLED_PREFIX = "nexus:publishers:"  # {prefix}{pub_id}:bidders:enabled (set)


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

    # =========================================================================
    # Publisher-specific bidder methods
    # =========================================================================

    def _pub_bidders_key(self, publisher_id: str) -> str:
        """Get Redis hash key for publisher-specific bidders."""
        return f"{REDIS_PUB_BIDDERS_PREFIX}{publisher_id}:bidders"

    def _pub_enabled_key(self, publisher_id: str) -> str:
        """Get Redis set key for publisher-enabled bidders."""
        return f"{REDIS_PUB_BIDDERS_ENABLED_PREFIX}{publisher_id}:bidders:enabled"

    def save_publisher_bidder(
        self, publisher_id: str, config: BidderConfig
    ) -> bool:
        """
        Save a publisher-specific bidder configuration.

        Args:
            publisher_id: The publisher identifier
            config: The bidder configuration to save

        Returns:
            True if saved successfully
        """
        if not self._redis:
            return False

        try:
            # Ensure publisher_id is set on config
            config.publisher_id = publisher_id
            config.updated_at = datetime.utcnow().isoformat()

            # Use pipeline for atomic operations
            pipe = self._redis.pipeline()

            # Store config in publisher-specific hash
            pub_hash_key = self._pub_bidders_key(publisher_id)
            pipe.hset(pub_hash_key, config.bidder_code, config.to_json())

            # Update enabled set
            pub_enabled_key = self._pub_enabled_key(publisher_id)
            if config.is_enabled:
                pipe.sadd(pub_enabled_key, config.bidder_code)
            else:
                pipe.srem(pub_enabled_key, config.bidder_code)

            pipe.execute()
            return True

        except Exception as e:
            print(f"Failed to save publisher bidder config: {e}")
            return False

    def get_publisher_bidder(
        self, publisher_id: str, bidder_code: str
    ) -> BidderConfig | None:
        """
        Get a publisher-specific bidder configuration.

        Args:
            publisher_id: The publisher identifier
            bidder_code: The unique bidder identifier

        Returns:
            BidderConfig if found, None otherwise
        """
        if not self._redis:
            return None

        try:
            pub_hash_key = self._pub_bidders_key(publisher_id)
            json_str = self._redis.hget(pub_hash_key, bidder_code)
            if json_str:
                return BidderConfig.from_json(json_str)
            return None
        except Exception as e:
            print(f"Failed to get publisher bidder config: {e}")
            return None

    def get_publisher_bidders(self, publisher_id: str) -> dict[str, BidderConfig]:
        """
        Get all publisher-specific bidder configurations.

        Args:
            publisher_id: The publisher identifier

        Returns:
            Dictionary mapping bidder_code to BidderConfig
        """
        if not self._redis:
            return {}

        try:
            pub_hash_key = self._pub_bidders_key(publisher_id)
            all_configs = self._redis.hgetall(pub_hash_key)
            result = {}
            for code, json_str in all_configs.items():
                try:
                    result[code] = BidderConfig.from_json(json_str)
                except Exception:
                    continue
            return result
        except Exception as e:
            print(f"Failed to get publisher bidder configs: {e}")
            return {}

    def delete_publisher_bidder(self, publisher_id: str, bidder_code: str) -> bool:
        """
        Delete a publisher-specific bidder configuration.

        Args:
            publisher_id: The publisher identifier
            bidder_code: The bidder identifier to delete

        Returns:
            True if deleted successfully
        """
        if not self._redis:
            return False

        try:
            pipe = self._redis.pipeline()
            pub_hash_key = self._pub_bidders_key(publisher_id)
            pub_enabled_key = self._pub_enabled_key(publisher_id)
            pipe.hdel(pub_hash_key, bidder_code)
            pipe.srem(pub_enabled_key, bidder_code)
            pipe.execute()
            return True
        except Exception as e:
            print(f"Failed to delete publisher bidder: {e}")
            return False

    def get_bidders_for_publisher(
        self, publisher_id: str, include_global: bool = True
    ) -> list[BidderConfig]:
        """
        Get all bidders available for a publisher.

        Combines global bidders with publisher-specific overrides.
        Publisher-specific bidders take precedence over global ones.

        Args:
            publisher_id: The publisher identifier
            include_global: Whether to include global bidders

        Returns:
            List of BidderConfig objects available for the publisher
        """
        result = {}

        # Start with global bidders if requested
        if include_global:
            for code, config in self.get_all().items():
                # Skip if publisher is blocked
                if (
                    config.blocked_publishers
                    and publisher_id in config.blocked_publishers
                ):
                    continue
                # Skip if allowed list exists and publisher not in it
                if (
                    config.allowed_publishers
                    and publisher_id not in config.allowed_publishers
                ):
                    continue
                result[code] = config

        # Overlay publisher-specific bidders
        pub_bidders = self.get_publisher_bidders(publisher_id)
        for code, config in pub_bidders.items():
            result[code] = config

        # Sort by priority and return as list
        sorted_configs = sorted(result.values(), key=lambda c: c.priority, reverse=True)
        return sorted_configs

    def get_enabled_bidders_for_publisher(self, publisher_id: str) -> set[str]:
        """
        Get the set of enabled bidder codes for a publisher.

        Args:
            publisher_id: The publisher identifier

        Returns:
            Set of enabled bidder codes
        """
        if not self._redis:
            return set()

        try:
            pub_enabled_key = self._pub_enabled_key(publisher_id)
            return self._redis.smembers(pub_enabled_key) or set()
        except Exception:
            return set()

    def set_bidder_enabled_for_publisher(
        self, publisher_id: str, bidder_code: str, enabled: bool
    ) -> bool:
        """
        Enable or disable a bidder for a specific publisher.

        This works for both global and publisher-specific bidders.

        Args:
            publisher_id: The publisher identifier
            bidder_code: The bidder code to enable/disable
            enabled: Whether to enable or disable

        Returns:
            True if updated successfully
        """
        if not self._redis:
            return False

        try:
            pub_enabled_key = self._pub_enabled_key(publisher_id)
            if enabled:
                self._redis.sadd(pub_enabled_key, bidder_code)
            else:
                self._redis.srem(pub_enabled_key, bidder_code)
            return True
        except Exception as e:
            print(f"Failed to set bidder enabled state: {e}")
            return False

    def generate_instance_code(
        self, publisher_id: str, bidder_family: str
    ) -> str:
        """
        Generate a new instance code for a bidder family.

        First instance: "appnexus"
        Second: "appnexus-2"
        Third: "appnexus-3"

        Args:
            publisher_id: The publisher identifier
            bidder_family: The base bidder family (e.g., "appnexus")

        Returns:
            The next available instance code
        """
        # Get existing codes for this family from both global and publisher-specific
        existing_codes = set()

        # Check global bidders
        for code in self.list_codes():
            family = BidderConfig._extract_family(code)
            if family == bidder_family:
                existing_codes.add(code)

        # Check publisher-specific bidders
        pub_bidders = self.get_publisher_bidders(publisher_id)
        for code in pub_bidders.keys():
            family = BidderConfig._extract_family(code)
            if family == bidder_family:
                existing_codes.add(code)

        if not existing_codes:
            return bidder_family

        # Find the max instance number
        max_num = 1
        for code in existing_codes:
            num = BidderConfig._extract_instance_number(code)
            if num > max_num:
                max_num = num

        return f"{bidder_family}-{max_num + 1}"

    def duplicate_bidder(
        self,
        publisher_id: str,
        source_bidder_code: str,
        new_name: str | None = None,
    ) -> BidderConfig | None:
        """
        Duplicate a bidder with auto-generated instance code.

        Args:
            publisher_id: The publisher identifier
            source_bidder_code: The bidder code to duplicate
            new_name: Optional new name (defaults to "Name (Copy)")

        Returns:
            The new BidderConfig if successful, None otherwise
        """
        # Try to get from publisher-specific first, then global
        source = self.get_publisher_bidder(publisher_id, source_bidder_code)
        if not source:
            source = self.get(source_bidder_code)
        if not source:
            return None

        # Generate new instance code
        new_code = self.generate_instance_code(publisher_id, source.bidder_family)

        # Create new config
        import copy
        new_config = copy.deepcopy(source)
        new_config.bidder_code = new_code
        new_config.bidder_family = source.bidder_family
        new_config.instance_number = BidderConfig._extract_instance_number(new_code)
        new_config.publisher_id = publisher_id
        new_config.name = new_name or f"{source.name} (Copy)"
        new_config.created_at = datetime.utcnow().isoformat()
        new_config.updated_at = new_config.created_at
        new_config.status = BidderStatus.TESTING

        # Reset stats
        new_config.total_requests = 0
        new_config.total_bids = 0
        new_config.total_wins = 0
        new_config.total_errors = 0
        new_config.avg_latency_ms = 0.0
        new_config.avg_bid_cpm = 0.0
        new_config.last_active_at = None

        # Save the new config
        if self.save_publisher_bidder(publisher_id, new_config):
            return new_config
        return None

    def get_bidder_families(self, publisher_id: str) -> dict[str, list[BidderConfig]]:
        """
        Get bidders grouped by family for a publisher.

        Args:
            publisher_id: The publisher identifier

        Returns:
            Dictionary mapping family to list of instances
        """
        bidders = self.get_bidders_for_publisher(publisher_id)
        families: dict[str, list[BidderConfig]] = {}

        for config in bidders:
            family = config.bidder_family
            if family not in families:
                families[family] = []
            families[family].append(config)

        # Sort instances within each family by instance_number
        for family in families:
            families[family].sort(key=lambda c: c.instance_number)

        return families


# Global storage instance
_bidder_storage: BidderStorage | None = None


def get_bidder_storage(redis_url: str | None = None) -> BidderStorage:
    """Get the global bidder storage instance."""
    global _bidder_storage

    if _bidder_storage is None:
        url = redis_url or os.environ.get("REDIS_URL", "redis://localhost:6379")
        _bidder_storage = BidderStorage(redis_url=url)

    return _bidder_storage
