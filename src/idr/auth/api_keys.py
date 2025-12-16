"""
API Key Management for The Nexus Engine

Provides unified API key management that syncs between IDR and PBS.
Keys are stored in Redis for fast validation by both services.

Usage:
    manager = APIKeyManager(redis_url="redis://localhost:6379")

    # Generate key for new publisher
    key = manager.generate_key("publisher-123")

    # Validate key (called by PBS)
    publisher_id = manager.validate_key("nxs_live_abc123...")

    # List all keys (admin)
    keys = manager.list_keys()
"""

import hashlib
import hmac
import os
import secrets
import time
from dataclasses import dataclass
from datetime import datetime
from typing import Optional

try:
    import redis
    REDIS_AVAILABLE = True
except ImportError:
    REDIS_AVAILABLE = False


# Key prefix for easy identification
KEY_PREFIX = "nxs"

# Redis key patterns
REDIS_API_KEYS_HASH = "nexus:api_keys"  # hash: api_key -> publisher_id
REDIS_PUBLISHER_KEYS = "nexus:publisher_keys"  # hash: publisher_id -> api_key
REDIS_KEY_METADATA = "nexus:key_meta"  # hash: api_key -> JSON metadata


@dataclass
class APIKeyInfo:
    """Information about an API key."""
    key: str
    publisher_id: str
    created_at: str
    last_used: Optional[str] = None
    enabled: bool = True
    request_count: int = 0

    @property
    def masked_key(self) -> str:
        """Return masked version of key for display."""
        if len(self.key) < 12:
            return "***"
        return f"{self.key[:8]}...{self.key[-4:]}"


class APIKeyManager:
    """
    Manages API keys for publisher authentication.

    Keys are stored in Redis for shared access between IDR and PBS services.
    PBS calls validate_key() on each request to authenticate publishers.
    """

    def __init__(
        self,
        redis_url: str = "redis://localhost:6379",
        key_prefix: str = KEY_PREFIX,
    ):
        """
        Initialize API key manager.

        Args:
            redis_url: Redis connection URL
            key_prefix: Prefix for generated keys (default: "nxs")
        """
        self.key_prefix = key_prefix
        self._redis: Optional[redis.Redis] = None
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

    def generate_key(
        self,
        publisher_id: str,
        environment: str = "live",
        replace_existing: bool = False,
    ) -> Optional[str]:
        """
        Generate a new API key for a publisher.

        Args:
            publisher_id: The publisher's unique identifier
            environment: "live" or "test" (affects key prefix)
            replace_existing: If True, revoke any existing key first

        Returns:
            The new API key, or None if generation failed
        """
        if not self._redis:
            return None

        # Check for existing key
        existing = self.get_publisher_key(publisher_id)
        if existing and not replace_existing:
            return existing

        if existing and replace_existing:
            self.revoke_key(existing)

        # Generate secure random key
        # Format: nxs_<env>_<random>
        random_part = secrets.token_urlsafe(32)
        env_prefix = "live" if environment == "live" else "test"
        api_key = f"{self.key_prefix}_{env_prefix}_{random_part}"

        # Store in Redis
        now = datetime.utcnow().isoformat()

        pipe = self._redis.pipeline()

        # Map key -> publisher
        pipe.hset(REDIS_API_KEYS_HASH, api_key, publisher_id)

        # Map publisher -> key
        pipe.hset(REDIS_PUBLISHER_KEYS, publisher_id, api_key)

        # Store metadata
        import json
        metadata = json.dumps({
            "publisher_id": publisher_id,
            "created_at": now,
            "environment": environment,
            "enabled": True,
            "request_count": 0,
        })
        pipe.hset(REDIS_KEY_METADATA, api_key, metadata)

        pipe.execute()

        return api_key

    def validate_key(self, api_key: str) -> Optional[str]:
        """
        Validate an API key and return the associated publisher ID.

        This is the fast path called by PBS on every request.

        Args:
            api_key: The API key to validate

        Returns:
            Publisher ID if valid, None if invalid
        """
        if not self._redis or not api_key:
            return None

        # Quick lookup
        publisher_id = self._redis.hget(REDIS_API_KEYS_HASH, api_key)

        if publisher_id:
            # Update last used timestamp (async, don't block)
            try:
                import json
                meta_json = self._redis.hget(REDIS_KEY_METADATA, api_key)
                if meta_json:
                    meta = json.loads(meta_json)
                    if not meta.get("enabled", True):
                        return None  # Key is disabled

                    # Update usage stats (fire and forget)
                    meta["last_used"] = datetime.utcnow().isoformat()
                    meta["request_count"] = meta.get("request_count", 0) + 1
                    self._redis.hset(REDIS_KEY_METADATA, api_key, json.dumps(meta))
            except Exception:
                pass  # Don't fail validation due to metadata update

        return publisher_id

    def get_publisher_key(self, publisher_id: str) -> Optional[str]:
        """Get the API key for a publisher."""
        if not self._redis:
            return None
        return self._redis.hget(REDIS_PUBLISHER_KEYS, publisher_id)

    def get_key_info(self, api_key: str) -> Optional[APIKeyInfo]:
        """Get detailed information about an API key."""
        if not self._redis:
            return None

        publisher_id = self._redis.hget(REDIS_API_KEYS_HASH, api_key)
        if not publisher_id:
            return None

        import json
        meta_json = self._redis.hget(REDIS_KEY_METADATA, api_key)
        meta = json.loads(meta_json) if meta_json else {}

        return APIKeyInfo(
            key=api_key,
            publisher_id=publisher_id,
            created_at=meta.get("created_at", ""),
            last_used=meta.get("last_used"),
            enabled=meta.get("enabled", True),
            request_count=meta.get("request_count", 0),
        )

    def revoke_key(self, api_key: str) -> bool:
        """
        Revoke an API key.

        Args:
            api_key: The key to revoke

        Returns:
            True if key was revoked, False if not found
        """
        if not self._redis:
            return False

        publisher_id = self._redis.hget(REDIS_API_KEYS_HASH, api_key)
        if not publisher_id:
            return False

        pipe = self._redis.pipeline()
        pipe.hdel(REDIS_API_KEYS_HASH, api_key)
        pipe.hdel(REDIS_PUBLISHER_KEYS, publisher_id)
        pipe.hdel(REDIS_KEY_METADATA, api_key)
        pipe.execute()

        return True

    def disable_key(self, api_key: str) -> bool:
        """Disable a key without revoking it."""
        if not self._redis:
            return False

        import json
        meta_json = self._redis.hget(REDIS_KEY_METADATA, api_key)
        if not meta_json:
            return False

        meta = json.loads(meta_json)
        meta["enabled"] = False
        self._redis.hset(REDIS_KEY_METADATA, api_key, json.dumps(meta))
        return True

    def enable_key(self, api_key: str) -> bool:
        """Re-enable a disabled key."""
        if not self._redis:
            return False

        import json
        meta_json = self._redis.hget(REDIS_KEY_METADATA, api_key)
        if not meta_json:
            return False

        meta = json.loads(meta_json)
        meta["enabled"] = True
        self._redis.hset(REDIS_KEY_METADATA, api_key, json.dumps(meta))
        return True

    def list_keys(self) -> list[APIKeyInfo]:
        """List all API keys with their info."""
        if not self._redis:
            return []

        keys = self._redis.hgetall(REDIS_API_KEYS_HASH)
        result = []

        for api_key, publisher_id in keys.items():
            info = self.get_key_info(api_key)
            if info:
                result.append(info)

        return result

    def sync_from_publisher_configs(self, configs: dict) -> int:
        """
        Sync API keys from publisher config files to Redis.

        This is called on startup to ensure Redis has all keys.

        Args:
            configs: Dict of publisher_id -> PublisherConfig

        Returns:
            Number of keys synced
        """
        if not self._redis:
            return 0

        synced = 0
        for pub_id, config in configs.items():
            if hasattr(config, 'api_key') and config.api_key.key:
                # Store existing key in Redis
                api_key = config.api_key.key

                pipe = self._redis.pipeline()
                pipe.hset(REDIS_API_KEYS_HASH, api_key, pub_id)
                pipe.hset(REDIS_PUBLISHER_KEYS, pub_id, api_key)

                import json
                metadata = json.dumps({
                    "publisher_id": pub_id,
                    "created_at": config.api_key.created_at or datetime.utcnow().isoformat(),
                    "enabled": config.api_key.enabled,
                    "request_count": 0,
                })
                pipe.hset(REDIS_KEY_METADATA, api_key, metadata)
                pipe.execute()
                synced += 1

        return synced


# Global instance
_api_key_manager: Optional[APIKeyManager] = None


def get_api_key_manager(redis_url: Optional[str] = None) -> APIKeyManager:
    """Get the global API key manager instance."""
    global _api_key_manager

    if _api_key_manager is None:
        url = redis_url or os.environ.get("REDIS_URL", "redis://localhost:6379")
        _api_key_manager = APIKeyManager(redis_url=url)

    return _api_key_manager
