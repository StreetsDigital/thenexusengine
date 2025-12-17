"""
Configuration Store - Redis-backed persistence for hierarchical configurations.

Persists all publisher, site, and ad unit configurations to Redis for
durability and sharing across multiple service instances.

Key Structure:
    nexus:config:global                     -> Global config JSON
    nexus:config:publishers                 -> Set of publisher IDs
    nexus:config:publisher:{pub_id}         -> Publisher config JSON
    nexus:config:sites:{pub_id}             -> Set of site IDs for publisher
    nexus:config:site:{pub_id}:{site_id}    -> Site config JSON
    nexus:config:ad_units:{pub_id}:{site_id} -> Set of ad unit IDs for site
    nexus:config:ad_unit:{pub_id}:{site_id}:{unit_id} -> Ad unit config JSON
"""

import json
import logging
import os
from datetime import datetime
from typing import Any, Optional

try:
    import redis
    REDIS_AVAILABLE = True
except ImportError:
    REDIS_AVAILABLE = False

from .feature_config import (
    ConfigLevel,
    FeatureConfig,
    get_default_global_config,
)
from .config_resolver import (
    AdUnitConfig,
    PublisherConfigV2,
    SiteConfig,
)


logger = logging.getLogger(__name__)


# Redis key prefixes
KEY_PREFIX = "nexus:config"
GLOBAL_KEY = f"{KEY_PREFIX}:global"
PUBLISHERS_SET_KEY = f"{KEY_PREFIX}:publishers"


def _publisher_key(publisher_id: str) -> str:
    """Get Redis key for a publisher config."""
    return f"{KEY_PREFIX}:publisher:{publisher_id}"


def _sites_set_key(publisher_id: str) -> str:
    """Get Redis key for a publisher's sites set."""
    return f"{KEY_PREFIX}:sites:{publisher_id}"


def _site_key(publisher_id: str, site_id: str) -> str:
    """Get Redis key for a site config."""
    return f"{KEY_PREFIX}:site:{publisher_id}:{site_id}"


def _ad_units_set_key(publisher_id: str, site_id: str) -> str:
    """Get Redis key for a site's ad units set."""
    return f"{KEY_PREFIX}:ad_units:{publisher_id}:{site_id}"


def _ad_unit_key(publisher_id: str, site_id: str, unit_id: str) -> str:
    """Get Redis key for an ad unit config."""
    return f"{KEY_PREFIX}:ad_unit:{publisher_id}:{site_id}:{unit_id}"


class ConfigStore:
    """
    Redis-backed configuration store.

    Provides persistence for all hierarchical configurations.
    Falls back to in-memory storage if Redis is unavailable.
    """

    def __init__(
        self,
        redis_url: Optional[str] = None,
        redis_client: Optional[Any] = None,
    ):
        """
        Initialize the config store.

        Args:
            redis_url: Redis connection URL (default: from REDIS_URL env var)
            redis_client: Optional pre-configured Redis client
        """
        self._redis: Optional[Any] = None
        self._memory_store: dict[str, Any] = {}
        self._use_memory = False

        if redis_client:
            self._redis = redis_client
        elif REDIS_AVAILABLE:
            url = redis_url or os.environ.get("REDIS_URL", "redis://localhost:6379/0")
            try:
                self._redis = redis.from_url(url, decode_responses=True)
                # Test connection
                self._redis.ping()
                logger.info(f"ConfigStore connected to Redis: {url}")
            except Exception as e:
                logger.warning(f"Redis connection failed, using memory store: {e}")
                self._use_memory = True
        else:
            logger.warning("Redis not available, using memory store")
            self._use_memory = True

    @property
    def is_redis_connected(self) -> bool:
        """Check if Redis is connected."""
        if self._use_memory:
            return False
        try:
            self._redis.ping()
            return True
        except Exception:
            return False

    # =========================================
    # Global Configuration
    # =========================================

    def save_global_config(self, config: FeatureConfig) -> bool:
        """Save the global configuration."""
        try:
            config.config_id = "global"
            config.config_level = ConfigLevel.GLOBAL
            config.updated_at = datetime.now()

            data = json.dumps(config.to_dict())

            if self._use_memory:
                self._memory_store[GLOBAL_KEY] = data
            else:
                self._redis.set(GLOBAL_KEY, data)

            logger.info("Global configuration saved")
            return True
        except Exception as e:
            logger.error(f"Failed to save global config: {e}")
            return False

    def load_global_config(self) -> FeatureConfig:
        """Load the global configuration."""
        try:
            if self._use_memory:
                data = self._memory_store.get(GLOBAL_KEY)
            else:
                data = self._redis.get(GLOBAL_KEY)

            if data:
                return FeatureConfig.from_dict(json.loads(data))
        except Exception as e:
            logger.error(f"Failed to load global config: {e}")

        return get_default_global_config()

    # =========================================
    # Publisher Configuration
    # =========================================

    def save_publisher(self, config: PublisherConfigV2) -> bool:
        """Save a publisher configuration."""
        try:
            config.features.config_id = f"publisher:{config.publisher_id}"
            config.features.config_level = ConfigLevel.PUBLISHER
            config.features.updated_at = datetime.now()

            # Prepare data (without nested sites - they're stored separately)
            data = {
                "publisher_id": config.publisher_id,
                "name": config.name,
                "enabled": config.enabled,
                "contact_email": config.contact_email,
                "contact_name": config.contact_name,
                "bidders": config.bidders,
                "api_key": config.api_key,
                "api_key_enabled": config.api_key_enabled,
                "features": config.features.to_dict(),
            }

            pub_key = _publisher_key(config.publisher_id)
            json_data = json.dumps(data)

            if self._use_memory:
                self._memory_store[pub_key] = json_data
                if PUBLISHERS_SET_KEY not in self._memory_store:
                    self._memory_store[PUBLISHERS_SET_KEY] = set()
                self._memory_store[PUBLISHERS_SET_KEY].add(config.publisher_id)
            else:
                pipe = self._redis.pipeline()
                pipe.set(pub_key, json_data)
                pipe.sadd(PUBLISHERS_SET_KEY, config.publisher_id)
                pipe.execute()

            # Save sites
            for site in config.sites:
                self.save_site(config.publisher_id, site)

            logger.info(f"Publisher {config.publisher_id} saved")
            return True
        except Exception as e:
            logger.error(f"Failed to save publisher {config.publisher_id}: {e}")
            return False

    def load_publisher(self, publisher_id: str) -> Optional[PublisherConfigV2]:
        """Load a publisher configuration."""
        try:
            pub_key = _publisher_key(publisher_id)

            if self._use_memory:
                data = self._memory_store.get(pub_key)
            else:
                data = self._redis.get(pub_key)

            if not data:
                return None

            pub_data = json.loads(data)

            # Load sites
            sites = self.load_sites(publisher_id)

            # Parse features
            features = FeatureConfig()
            if "features" in pub_data:
                features = FeatureConfig.from_dict(pub_data["features"])

            return PublisherConfigV2(
                publisher_id=pub_data.get("publisher_id", publisher_id),
                name=pub_data.get("name", ""),
                enabled=pub_data.get("enabled", True),
                contact_email=pub_data.get("contact_email", ""),
                contact_name=pub_data.get("contact_name", ""),
                bidders=pub_data.get("bidders", {}),
                api_key=pub_data.get("api_key", ""),
                api_key_enabled=pub_data.get("api_key_enabled", True),
                sites=sites,
                features=features,
            )
        except Exception as e:
            logger.error(f"Failed to load publisher {publisher_id}: {e}")
            return None

    def delete_publisher(self, publisher_id: str) -> bool:
        """Delete a publisher and all its sites/ad units."""
        try:
            # Delete all sites first
            sites = self.load_sites(publisher_id)
            for site in sites:
                self.delete_site(publisher_id, site.site_id)

            pub_key = _publisher_key(publisher_id)
            sites_set_key = _sites_set_key(publisher_id)

            if self._use_memory:
                self._memory_store.pop(pub_key, None)
                self._memory_store.pop(sites_set_key, None)
                if PUBLISHERS_SET_KEY in self._memory_store:
                    self._memory_store[PUBLISHERS_SET_KEY].discard(publisher_id)
            else:
                pipe = self._redis.pipeline()
                pipe.delete(pub_key)
                pipe.delete(sites_set_key)
                pipe.srem(PUBLISHERS_SET_KEY, publisher_id)
                pipe.execute()

            logger.info(f"Publisher {publisher_id} deleted")
            return True
        except Exception as e:
            logger.error(f"Failed to delete publisher {publisher_id}: {e}")
            return False

    def list_publishers(self) -> list[str]:
        """List all publisher IDs."""
        try:
            if self._use_memory:
                return list(self._memory_store.get(PUBLISHERS_SET_KEY, set()))
            else:
                return list(self._redis.smembers(PUBLISHERS_SET_KEY))
        except Exception as e:
            logger.error(f"Failed to list publishers: {e}")
            return []

    def load_all_publishers(self) -> dict[str, PublisherConfigV2]:
        """Load all publisher configurations."""
        publishers = {}
        for pub_id in self.list_publishers():
            config = self.load_publisher(pub_id)
            if config:
                publishers[pub_id] = config
        return publishers

    # =========================================
    # Site Configuration
    # =========================================

    def save_site(self, publisher_id: str, config: SiteConfig) -> bool:
        """Save a site configuration."""
        try:
            config.features.config_id = f"site:{config.site_id}"
            config.features.config_level = ConfigLevel.SITE
            config.features.updated_at = datetime.now()

            # Prepare data (without nested ad_units - they're stored separately)
            data = {
                "site_id": config.site_id,
                "domain": config.domain,
                "name": config.name,
                "enabled": config.enabled,
                "features": config.features.to_dict(),
            }

            site_key = _site_key(publisher_id, config.site_id)
            sites_set = _sites_set_key(publisher_id)
            json_data = json.dumps(data)

            if self._use_memory:
                self._memory_store[site_key] = json_data
                if sites_set not in self._memory_store:
                    self._memory_store[sites_set] = set()
                self._memory_store[sites_set].add(config.site_id)
            else:
                pipe = self._redis.pipeline()
                pipe.set(site_key, json_data)
                pipe.sadd(sites_set, config.site_id)
                pipe.execute()

            # Save ad units
            for ad_unit in config.ad_units:
                self.save_ad_unit(publisher_id, config.site_id, ad_unit)

            logger.debug(f"Site {config.site_id} saved for publisher {publisher_id}")
            return True
        except Exception as e:
            logger.error(f"Failed to save site {config.site_id}: {e}")
            return False

    def load_site(self, publisher_id: str, site_id: str) -> Optional[SiteConfig]:
        """Load a site configuration."""
        try:
            site_key = _site_key(publisher_id, site_id)

            if self._use_memory:
                data = self._memory_store.get(site_key)
            else:
                data = self._redis.get(site_key)

            if not data:
                return None

            site_data = json.loads(data)

            # Load ad units
            ad_units = self.load_ad_units(publisher_id, site_id)

            # Parse features
            features = FeatureConfig()
            if "features" in site_data:
                features = FeatureConfig.from_dict(site_data["features"])

            return SiteConfig(
                site_id=site_data.get("site_id", site_id),
                domain=site_data.get("domain", ""),
                name=site_data.get("name", ""),
                enabled=site_data.get("enabled", True),
                ad_units=ad_units,
                features=features,
            )
        except Exception as e:
            logger.error(f"Failed to load site {site_id}: {e}")
            return None

    def delete_site(self, publisher_id: str, site_id: str) -> bool:
        """Delete a site and all its ad units."""
        try:
            # Delete all ad units first
            ad_units = self.load_ad_units(publisher_id, site_id)
            for ad_unit in ad_units:
                self.delete_ad_unit(publisher_id, site_id, ad_unit.unit_id)

            site_key = _site_key(publisher_id, site_id)
            sites_set = _sites_set_key(publisher_id)
            units_set = _ad_units_set_key(publisher_id, site_id)

            if self._use_memory:
                self._memory_store.pop(site_key, None)
                self._memory_store.pop(units_set, None)
                if sites_set in self._memory_store:
                    self._memory_store[sites_set].discard(site_id)
            else:
                pipe = self._redis.pipeline()
                pipe.delete(site_key)
                pipe.delete(units_set)
                pipe.srem(sites_set, site_id)
                pipe.execute()

            logger.debug(f"Site {site_id} deleted")
            return True
        except Exception as e:
            logger.error(f"Failed to delete site {site_id}: {e}")
            return False

    def load_sites(self, publisher_id: str) -> list[SiteConfig]:
        """Load all sites for a publisher."""
        sites = []
        try:
            sites_set = _sites_set_key(publisher_id)

            if self._use_memory:
                site_ids = self._memory_store.get(sites_set, set())
            else:
                site_ids = self._redis.smembers(sites_set)

            for site_id in site_ids:
                site = self.load_site(publisher_id, site_id)
                if site:
                    sites.append(site)
        except Exception as e:
            logger.error(f"Failed to load sites for publisher {publisher_id}: {e}")

        return sites

    # =========================================
    # Ad Unit Configuration
    # =========================================

    def save_ad_unit(self, publisher_id: str, site_id: str, config: AdUnitConfig) -> bool:
        """Save an ad unit configuration."""
        try:
            config.features.config_id = f"ad_unit:{config.unit_id}"
            config.features.config_level = ConfigLevel.AD_UNIT
            config.features.updated_at = datetime.now()

            data = {
                "unit_id": config.unit_id,
                "name": config.name,
                "enabled": config.enabled,
                "sizes": config.sizes,
                "media_type": config.media_type,
                "position": config.position,
                "floor_price": config.floor_price,
                "floor_currency": config.floor_currency,
                "video": config.video,
                "features": config.features.to_dict(),
            }

            unit_key = _ad_unit_key(publisher_id, site_id, config.unit_id)
            units_set = _ad_units_set_key(publisher_id, site_id)
            json_data = json.dumps(data)

            if self._use_memory:
                self._memory_store[unit_key] = json_data
                if units_set not in self._memory_store:
                    self._memory_store[units_set] = set()
                self._memory_store[units_set].add(config.unit_id)
            else:
                pipe = self._redis.pipeline()
                pipe.set(unit_key, json_data)
                pipe.sadd(units_set, config.unit_id)
                pipe.execute()

            logger.debug(f"Ad unit {config.unit_id} saved")
            return True
        except Exception as e:
            logger.error(f"Failed to save ad unit {config.unit_id}: {e}")
            return False

    def load_ad_unit(
        self, publisher_id: str, site_id: str, unit_id: str
    ) -> Optional[AdUnitConfig]:
        """Load an ad unit configuration."""
        try:
            unit_key = _ad_unit_key(publisher_id, site_id, unit_id)

            if self._use_memory:
                data = self._memory_store.get(unit_key)
            else:
                data = self._redis.get(unit_key)

            if not data:
                return None

            unit_data = json.loads(data)

            # Parse features
            features = FeatureConfig()
            if "features" in unit_data:
                features = FeatureConfig.from_dict(unit_data["features"])

            return AdUnitConfig(
                unit_id=unit_data.get("unit_id", unit_id),
                name=unit_data.get("name", ""),
                enabled=unit_data.get("enabled", True),
                sizes=unit_data.get("sizes", []),
                media_type=unit_data.get("media_type", "banner"),
                position=unit_data.get("position", "unknown"),
                floor_price=unit_data.get("floor_price"),
                floor_currency=unit_data.get("floor_currency", "USD"),
                video=unit_data.get("video"),
                features=features,
            )
        except Exception as e:
            logger.error(f"Failed to load ad unit {unit_id}: {e}")
            return None

    def delete_ad_unit(self, publisher_id: str, site_id: str, unit_id: str) -> bool:
        """Delete an ad unit."""
        try:
            unit_key = _ad_unit_key(publisher_id, site_id, unit_id)
            units_set = _ad_units_set_key(publisher_id, site_id)

            if self._use_memory:
                self._memory_store.pop(unit_key, None)
                if units_set in self._memory_store:
                    self._memory_store[units_set].discard(unit_id)
            else:
                pipe = self._redis.pipeline()
                pipe.delete(unit_key)
                pipe.srem(units_set, unit_id)
                pipe.execute()

            logger.debug(f"Ad unit {unit_id} deleted")
            return True
        except Exception as e:
            logger.error(f"Failed to delete ad unit {unit_id}: {e}")
            return False

    def load_ad_units(self, publisher_id: str, site_id: str) -> list[AdUnitConfig]:
        """Load all ad units for a site."""
        ad_units = []
        try:
            units_set = _ad_units_set_key(publisher_id, site_id)

            if self._use_memory:
                unit_ids = self._memory_store.get(units_set, set())
            else:
                unit_ids = self._redis.smembers(units_set)

            for unit_id in unit_ids:
                ad_unit = self.load_ad_unit(publisher_id, site_id, unit_id)
                if ad_unit:
                    ad_units.append(ad_unit)
        except Exception as e:
            logger.error(f"Failed to load ad units for site {site_id}: {e}")

        return ad_units

    # =========================================
    # Bulk Operations
    # =========================================

    def save_all(
        self,
        global_config: Optional[FeatureConfig] = None,
        publishers: Optional[dict[str, PublisherConfigV2]] = None,
    ) -> bool:
        """Save all configurations at once."""
        success = True

        if global_config:
            if not self.save_global_config(global_config):
                success = False

        if publishers:
            for config in publishers.values():
                if not self.save_publisher(config):
                    success = False

        return success

    def load_all(self) -> tuple[FeatureConfig, dict[str, PublisherConfigV2]]:
        """Load all configurations."""
        global_config = self.load_global_config()
        publishers = self.load_all_publishers()
        return global_config, publishers

    def export_to_dict(self) -> dict[str, Any]:
        """Export all configurations to a dictionary."""
        global_config, publishers = self.load_all()

        return {
            "global": global_config.to_dict(),
            "publishers": {
                pub_id: config.to_dict()
                for pub_id, config in publishers.items()
            },
        }

    def import_from_dict(self, data: dict[str, Any]) -> bool:
        """Import configurations from a dictionary."""
        success = True

        # Import global config
        if "global" in data:
            config = FeatureConfig.from_dict(data["global"])
            if not self.save_global_config(config):
                success = False

        # Import publishers
        for pub_id, pub_data in data.get("publishers", {}).items():
            config = _parse_publisher_from_dict(pub_data)
            if not self.save_publisher(config):
                success = False

        return success

    def clear_all(self) -> bool:
        """Clear all configurations (use with caution!)."""
        try:
            if self._use_memory:
                self._memory_store.clear()
            else:
                # Get all keys with our prefix
                keys = self._redis.keys(f"{KEY_PREFIX}:*")
                if keys:
                    self._redis.delete(*keys)

            logger.warning("All configurations cleared")
            return True
        except Exception as e:
            logger.error(f"Failed to clear configurations: {e}")
            return False


def _parse_publisher_from_dict(data: dict[str, Any]) -> PublisherConfigV2:
    """Parse publisher config from dictionary."""
    # Parse features
    features = FeatureConfig()
    if "features" in data:
        features = FeatureConfig.from_dict(data["features"])

    # Parse sites
    sites = []
    for site_data in data.get("sites", []):
        sites.append(_parse_site_from_dict(site_data))

    return PublisherConfigV2(
        publisher_id=data.get("publisher_id", ""),
        name=data.get("name", ""),
        enabled=data.get("enabled", True),
        contact_email=data.get("contact_email", ""),
        contact_name=data.get("contact_name", ""),
        bidders=data.get("bidders", {}),
        api_key=data.get("api_key", ""),
        api_key_enabled=data.get("api_key_enabled", True),
        sites=sites,
        features=features,
    )


def _parse_site_from_dict(data: dict[str, Any]) -> SiteConfig:
    """Parse site config from dictionary."""
    # Parse features
    features = FeatureConfig()
    if "features" in data:
        features = FeatureConfig.from_dict(data["features"])

    # Parse ad units
    ad_units = []
    for unit_data in data.get("ad_units", []):
        ad_units.append(_parse_ad_unit_from_dict(unit_data))

    return SiteConfig(
        site_id=data.get("site_id", ""),
        domain=data.get("domain", ""),
        name=data.get("name", ""),
        enabled=data.get("enabled", True),
        ad_units=ad_units,
        features=features,
    )


def _parse_ad_unit_from_dict(data: dict[str, Any]) -> AdUnitConfig:
    """Parse ad unit config from dictionary."""
    # Parse features
    features = FeatureConfig()
    if "features" in data:
        features = FeatureConfig.from_dict(data["features"])

    return AdUnitConfig(
        unit_id=data.get("unit_id", ""),
        name=data.get("name", ""),
        enabled=data.get("enabled", True),
        sizes=data.get("sizes", []),
        media_type=data.get("media_type", "banner"),
        position=data.get("position", "unknown"),
        floor_price=data.get("floor_price"),
        floor_currency=data.get("floor_currency", "USD"),
        video=data.get("video"),
        features=features,
    )


# Global config store instance
_store: Optional[ConfigStore] = None


def get_config_store() -> ConfigStore:
    """Get the global config store instance."""
    global _store
    if _store is None:
        _store = ConfigStore()
    return _store


def init_config_store(redis_url: Optional[str] = None) -> ConfigStore:
    """Initialize the global config store with a specific Redis URL."""
    global _store
    _store = ConfigStore(redis_url=redis_url)
    return _store
