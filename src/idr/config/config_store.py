"""
Configuration Store - PostgreSQL-backed persistence for hierarchical configurations.

Persists all publisher, site, and ad unit configurations to PostgreSQL for
durability and sharing across multiple service instances.

Tables:
    config_global       -> Global config JSON
    config_publishers   -> Publisher configurations
    config_sites        -> Site configurations (per publisher)
    config_ad_units     -> Ad unit configurations (per site)
"""

import json
import logging
import os
from datetime import datetime
from typing import Any

try:
    import psycopg2
    import psycopg2.extras

    PSYCOPG2_AVAILABLE = True
except ImportError:
    PSYCOPG2_AVAILABLE = False

from .config_resolver import (
    AdUnitConfig,
    PublisherConfigV2,
    SiteConfig,
)
from .feature_config import (
    ConfigLevel,
    FeatureConfig,
    get_default_global_config,
)

logger = logging.getLogger(__name__)


class ConfigStore:
    """
    PostgreSQL-backed configuration store.

    Provides persistence for all hierarchical configurations.
    Falls back to in-memory storage if PostgreSQL is unavailable.
    """

    def __init__(
        self,
        host: str | None = None,
        port: int | None = None,
        database: str | None = None,
        user: str | None = None,
        password: str | None = None,
        database_url: str | None = None,
    ):
        """
        Initialize the config store.

        Args:
            host: PostgreSQL host (default: from POSTGRES_HOST env var or localhost)
            port: PostgreSQL port (default: from POSTGRES_PORT env var or 5432)
            database: Database name (default: from POSTGRES_DB env var or idr)
            user: Database user (default: from POSTGRES_USER env var or idr)
            password: Database password (default: from POSTGRES_PASSWORD env var)
            database_url: Full connection URL (overrides individual params)
        """
        self._conn: Any | None = None
        self._memory_store: dict[str, Any] = {}
        self._use_memory = False

        if not PSYCOPG2_AVAILABLE:
            logger.warning("psycopg2 not available, using memory store")
            self._use_memory = True
            return

        # Build connection params from environment or arguments
        if database_url:
            self._database_url = database_url
            self._connection_params = None
        else:
            self._database_url = os.environ.get("DATABASE_URL")
            if self._database_url:
                self._connection_params = None
            else:
                self._connection_params = {
                    "host": host or os.environ.get("POSTGRES_HOST", "localhost"),
                    "port": port or int(os.environ.get("POSTGRES_PORT", "5432")),
                    "database": database or os.environ.get("POSTGRES_DB", "idr"),
                    "user": user or os.environ.get("POSTGRES_USER", "idr"),
                    "password": password or os.environ.get("POSTGRES_PASSWORD", ""),
                }

        # Try to connect
        if not self._connect():
            logger.warning("PostgreSQL connection failed, using memory store")
            self._use_memory = True

    def _connect(self) -> bool:
        """Establish database connection."""
        try:
            if self._database_url:
                self._conn = psycopg2.connect(self._database_url)
            else:
                self._conn = psycopg2.connect(**self._connection_params)
            self._conn.autocommit = False
            logger.info("ConfigStore connected to PostgreSQL")
            return True
        except Exception as e:
            logger.error(f"Failed to connect to PostgreSQL: {e}")
            return False

    def _ensure_connected(self) -> bool:
        """Ensure we have a valid connection, reconnect if needed."""
        if self._use_memory:
            return True

        try:
            if self._conn and not self._conn.closed:
                # Test connection
                with self._conn.cursor() as cur:
                    cur.execute("SELECT 1")
                return True
        except Exception:
            pass

        # Try to reconnect
        return self._connect()

    @property
    def is_connected(self) -> bool:
        """Check if PostgreSQL is connected."""
        if self._use_memory:
            return False
        try:
            if self._conn and not self._conn.closed:
                with self._conn.cursor() as cur:
                    cur.execute("SELECT 1")
                return True
        except Exception:
            pass
        return False

    def close(self) -> None:
        """Close the database connection."""
        if self._conn:
            self._conn.close()
            self._conn = None

    # =========================================
    # Global Configuration
    # =========================================

    def save_global_config(self, config: FeatureConfig) -> bool:
        """Save the global configuration."""
        try:
            config.config_id = "global"
            config.config_level = ConfigLevel.GLOBAL
            config.updated_at = datetime.now()

            data = config.to_dict()

            if self._use_memory:
                self._memory_store["global"] = data
                return True

            if not self._ensure_connected():
                return False

            with self._conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO config_global (id, config)
                    VALUES (1, %s)
                    ON CONFLICT (id) DO UPDATE SET config = EXCLUDED.config
                """,
                    (json.dumps(data),),
                )
            self._conn.commit()

            logger.info("Global configuration saved")
            return True
        except Exception as e:
            logger.error(f"Failed to save global config: {e}")
            if self._conn:
                self._conn.rollback()
            return False

    def load_global_config(self) -> FeatureConfig:
        """Load the global configuration."""
        try:
            if self._use_memory:
                data = self._memory_store.get("global")
                if data:
                    return FeatureConfig.from_dict(data)
                return get_default_global_config()

            if not self._ensure_connected():
                return get_default_global_config()

            with self._conn.cursor() as cur:
                cur.execute("SELECT config FROM config_global WHERE id = 1")
                row = cur.fetchone()
                if row:
                    return FeatureConfig.from_dict(row[0])
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

            if self._use_memory:
                self._memory_store[f"publisher:{config.publisher_id}"] = {
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
                # Save sites
                for site in config.sites:
                    self.save_site(config.publisher_id, site)
                return True

            if not self._ensure_connected():
                return False

            with self._conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO config_publishers (
                        publisher_id, name, enabled, contact_email, contact_name,
                        bidders, api_key, api_key_enabled, features
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (publisher_id) DO UPDATE SET
                        name = EXCLUDED.name,
                        enabled = EXCLUDED.enabled,
                        contact_email = EXCLUDED.contact_email,
                        contact_name = EXCLUDED.contact_name,
                        bidders = EXCLUDED.bidders,
                        api_key = EXCLUDED.api_key,
                        api_key_enabled = EXCLUDED.api_key_enabled,
                        features = EXCLUDED.features
                """,
                    (
                        config.publisher_id,
                        config.name,
                        config.enabled,
                        config.contact_email,
                        config.contact_name,
                        json.dumps(config.bidders),
                        config.api_key,
                        config.api_key_enabled,
                        json.dumps(config.features.to_dict()),
                    ),
                )
            self._conn.commit()

            # Save sites
            for site in config.sites:
                self.save_site(config.publisher_id, site)

            logger.info(f"Publisher {config.publisher_id} saved")
            return True
        except Exception as e:
            logger.error(f"Failed to save publisher {config.publisher_id}: {e}")
            if self._conn:
                self._conn.rollback()
            return False

    def load_publisher(self, publisher_id: str) -> PublisherConfigV2 | None:
        """Load a publisher configuration."""
        try:
            if self._use_memory:
                data = self._memory_store.get(f"publisher:{publisher_id}")
                if not data:
                    return None
                sites = self.load_sites(publisher_id)
                features = FeatureConfig()
                if "features" in data:
                    features = FeatureConfig.from_dict(data["features"])
                return PublisherConfigV2(
                    publisher_id=data.get("publisher_id", publisher_id),
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

            if not self._ensure_connected():
                return None

            with self._conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
                cur.execute(
                    """
                    SELECT publisher_id, name, enabled, contact_email, contact_name,
                           bidders, api_key, api_key_enabled, features
                    FROM config_publishers
                    WHERE publisher_id = %s
                """,
                    (publisher_id,),
                )
                row = cur.fetchone()

                if not row:
                    return None

                # Load sites
                sites = self.load_sites(publisher_id)

                # Parse features
                features = FeatureConfig()
                if row["features"]:
                    features = FeatureConfig.from_dict(row["features"])

                return PublisherConfigV2(
                    publisher_id=row["publisher_id"],
                    name=row["name"] or "",
                    enabled=row["enabled"],
                    contact_email=row["contact_email"] or "",
                    contact_name=row["contact_name"] or "",
                    bidders=row["bidders"] or {},
                    api_key=row["api_key"] or "",
                    api_key_enabled=row["api_key_enabled"],
                    sites=sites,
                    features=features,
                )
        except Exception as e:
            logger.error(f"Failed to load publisher {publisher_id}: {e}")
            return None

    def delete_publisher(self, publisher_id: str) -> bool:
        """Delete a publisher and all its sites/ad units."""
        try:
            if self._use_memory:
                # Delete sites first
                sites = self.load_sites(publisher_id)
                for site in sites:
                    self.delete_site(publisher_id, site.site_id)
                self._memory_store.pop(f"publisher:{publisher_id}", None)
                return True

            if not self._ensure_connected():
                return False

            # CASCADE will handle sites and ad units
            with self._conn.cursor() as cur:
                cur.execute(
                    "DELETE FROM config_publishers WHERE publisher_id = %s",
                    (publisher_id,),
                )
            self._conn.commit()

            logger.info(f"Publisher {publisher_id} deleted")
            return True
        except Exception as e:
            logger.error(f"Failed to delete publisher {publisher_id}: {e}")
            if self._conn:
                self._conn.rollback()
            return False

    def list_publishers(self) -> list[str]:
        """List all publisher IDs."""
        try:
            if self._use_memory:
                return [
                    k.replace("publisher:", "")
                    for k in self._memory_store.keys()
                    if k.startswith("publisher:")
                ]

            if not self._ensure_connected():
                return []

            with self._conn.cursor() as cur:
                cur.execute("SELECT publisher_id FROM config_publishers ORDER BY name")
                return [row[0] for row in cur.fetchall()]
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

            if self._use_memory:
                key = f"site:{publisher_id}:{config.site_id}"
                self._memory_store[key] = {
                    "site_id": config.site_id,
                    "publisher_id": publisher_id,
                    "domain": config.domain,
                    "name": config.name,
                    "enabled": config.enabled,
                    "features": config.features.to_dict(),
                }
                # Save ad units
                for ad_unit in config.ad_units:
                    self.save_ad_unit(publisher_id, config.site_id, ad_unit)
                return True

            if not self._ensure_connected():
                return False

            with self._conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO config_sites (
                        site_id, publisher_id, domain, name, enabled, features
                    ) VALUES (%s, %s, %s, %s, %s, %s)
                    ON CONFLICT (publisher_id, site_id) DO UPDATE SET
                        domain = EXCLUDED.domain,
                        name = EXCLUDED.name,
                        enabled = EXCLUDED.enabled,
                        features = EXCLUDED.features
                """,
                    (
                        config.site_id,
                        publisher_id,
                        config.domain,
                        config.name,
                        config.enabled,
                        json.dumps(config.features.to_dict()),
                    ),
                )
            self._conn.commit()

            # Save ad units
            for ad_unit in config.ad_units:
                self.save_ad_unit(publisher_id, config.site_id, ad_unit)

            logger.debug(f"Site {config.site_id} saved for publisher {publisher_id}")
            return True
        except Exception as e:
            logger.error(f"Failed to save site {config.site_id}: {e}")
            if self._conn:
                self._conn.rollback()
            return False

    def load_site(self, publisher_id: str, site_id: str) -> SiteConfig | None:
        """Load a site configuration."""
        try:
            if self._use_memory:
                key = f"site:{publisher_id}:{site_id}"
                data = self._memory_store.get(key)
                if not data:
                    return None
                ad_units = self.load_ad_units(publisher_id, site_id)
                features = FeatureConfig()
                if "features" in data:
                    features = FeatureConfig.from_dict(data["features"])
                return SiteConfig(
                    site_id=data.get("site_id", site_id),
                    domain=data.get("domain", ""),
                    name=data.get("name", ""),
                    enabled=data.get("enabled", True),
                    ad_units=ad_units,
                    features=features,
                )

            if not self._ensure_connected():
                return None

            with self._conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
                cur.execute(
                    """
                    SELECT site_id, domain, name, enabled, features
                    FROM config_sites
                    WHERE publisher_id = %s AND site_id = %s
                """,
                    (publisher_id, site_id),
                )
                row = cur.fetchone()

                if not row:
                    return None

                # Load ad units
                ad_units = self.load_ad_units(publisher_id, site_id)

                # Parse features
                features = FeatureConfig()
                if row["features"]:
                    features = FeatureConfig.from_dict(row["features"])

                return SiteConfig(
                    site_id=row["site_id"],
                    domain=row["domain"] or "",
                    name=row["name"] or "",
                    enabled=row["enabled"],
                    ad_units=ad_units,
                    features=features,
                )
        except Exception as e:
            logger.error(f"Failed to load site {site_id}: {e}")
            return None

    def delete_site(self, publisher_id: str, site_id: str) -> bool:
        """Delete a site and all its ad units."""
        try:
            if self._use_memory:
                # Delete ad units first
                ad_units = self.load_ad_units(publisher_id, site_id)
                for ad_unit in ad_units:
                    self.delete_ad_unit(publisher_id, site_id, ad_unit.unit_id)
                key = f"site:{publisher_id}:{site_id}"
                self._memory_store.pop(key, None)
                return True

            if not self._ensure_connected():
                return False

            # CASCADE will handle ad units
            with self._conn.cursor() as cur:
                cur.execute(
                    "DELETE FROM config_sites WHERE publisher_id = %s AND site_id = %s",
                    (publisher_id, site_id),
                )
            self._conn.commit()

            logger.debug(f"Site {site_id} deleted")
            return True
        except Exception as e:
            logger.error(f"Failed to delete site {site_id}: {e}")
            if self._conn:
                self._conn.rollback()
            return False

    def load_sites(self, publisher_id: str) -> list[SiteConfig]:
        """Load all sites for a publisher."""
        sites = []
        try:
            if self._use_memory:
                prefix = f"site:{publisher_id}:"
                site_ids = [
                    k.replace(prefix, "")
                    for k in self._memory_store.keys()
                    if k.startswith(prefix)
                ]
                for site_id in site_ids:
                    site = self.load_site(publisher_id, site_id)
                    if site:
                        sites.append(site)
                return sites

            if not self._ensure_connected():
                return sites

            with self._conn.cursor() as cur:
                cur.execute(
                    "SELECT site_id FROM config_sites WHERE publisher_id = %s ORDER BY name",
                    (publisher_id,),
                )
                site_ids = [row[0] for row in cur.fetchall()]

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

    def save_ad_unit(
        self, publisher_id: str, site_id: str, config: AdUnitConfig
    ) -> bool:
        """Save an ad unit configuration."""
        try:
            config.features.config_id = f"ad_unit:{config.unit_id}"
            config.features.config_level = ConfigLevel.AD_UNIT
            config.features.updated_at = datetime.now()

            if self._use_memory:
                key = f"ad_unit:{publisher_id}:{site_id}:{config.unit_id}"
                self._memory_store[key] = {
                    "unit_id": config.unit_id,
                    "publisher_id": publisher_id,
                    "site_id": site_id,
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
                return True

            if not self._ensure_connected():
                return False

            with self._conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO config_ad_units (
                        unit_id, site_id, publisher_id, name, enabled, sizes,
                        media_type, position, floor_price, floor_currency, video, features
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (publisher_id, site_id, unit_id) DO UPDATE SET
                        name = EXCLUDED.name,
                        enabled = EXCLUDED.enabled,
                        sizes = EXCLUDED.sizes,
                        media_type = EXCLUDED.media_type,
                        position = EXCLUDED.position,
                        floor_price = EXCLUDED.floor_price,
                        floor_currency = EXCLUDED.floor_currency,
                        video = EXCLUDED.video,
                        features = EXCLUDED.features
                """,
                    (
                        config.unit_id,
                        site_id,
                        publisher_id,
                        config.name,
                        config.enabled,
                        json.dumps(config.sizes),
                        config.media_type,
                        config.position,
                        config.floor_price,
                        config.floor_currency,
                        json.dumps(config.video) if config.video else None,
                        json.dumps(config.features.to_dict()),
                    ),
                )
            self._conn.commit()

            logger.debug(f"Ad unit {config.unit_id} saved")
            return True
        except Exception as e:
            logger.error(f"Failed to save ad unit {config.unit_id}: {e}")
            if self._conn:
                self._conn.rollback()
            return False

    def load_ad_unit(
        self, publisher_id: str, site_id: str, unit_id: str
    ) -> AdUnitConfig | None:
        """Load an ad unit configuration."""
        try:
            if self._use_memory:
                key = f"ad_unit:{publisher_id}:{site_id}:{unit_id}"
                data = self._memory_store.get(key)
                if not data:
                    return None
                features = FeatureConfig()
                if "features" in data:
                    features = FeatureConfig.from_dict(data["features"])
                return AdUnitConfig(
                    unit_id=data.get("unit_id", unit_id),
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

            if not self._ensure_connected():
                return None

            with self._conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
                cur.execute(
                    """
                    SELECT unit_id, name, enabled, sizes, media_type, position,
                           floor_price, floor_currency, video, features
                    FROM config_ad_units
                    WHERE publisher_id = %s AND site_id = %s AND unit_id = %s
                """,
                    (publisher_id, site_id, unit_id),
                )
                row = cur.fetchone()

                if not row:
                    return None

                # Parse features
                features = FeatureConfig()
                if row["features"]:
                    features = FeatureConfig.from_dict(row["features"])

                return AdUnitConfig(
                    unit_id=row["unit_id"],
                    name=row["name"] or "",
                    enabled=row["enabled"],
                    sizes=row["sizes"] or [],
                    media_type=row["media_type"] or "banner",
                    position=row["position"] or "unknown",
                    floor_price=float(row["floor_price"])
                    if row["floor_price"]
                    else None,
                    floor_currency=row["floor_currency"] or "USD",
                    video=row["video"],
                    features=features,
                )
        except Exception as e:
            logger.error(f"Failed to load ad unit {unit_id}: {e}")
            return None

    def delete_ad_unit(self, publisher_id: str, site_id: str, unit_id: str) -> bool:
        """Delete an ad unit."""
        try:
            if self._use_memory:
                key = f"ad_unit:{publisher_id}:{site_id}:{unit_id}"
                self._memory_store.pop(key, None)
                return True

            if not self._ensure_connected():
                return False

            with self._conn.cursor() as cur:
                cur.execute(
                    "DELETE FROM config_ad_units WHERE publisher_id = %s AND site_id = %s AND unit_id = %s",
                    (publisher_id, site_id, unit_id),
                )
            self._conn.commit()

            logger.debug(f"Ad unit {unit_id} deleted")
            return True
        except Exception as e:
            logger.error(f"Failed to delete ad unit {unit_id}: {e}")
            if self._conn:
                self._conn.rollback()
            return False

    def load_ad_units(self, publisher_id: str, site_id: str) -> list[AdUnitConfig]:
        """Load all ad units for a site."""
        ad_units = []
        try:
            if self._use_memory:
                prefix = f"ad_unit:{publisher_id}:{site_id}:"
                unit_ids = [
                    k.replace(prefix, "")
                    for k in self._memory_store.keys()
                    if k.startswith(prefix)
                ]
                for unit_id in unit_ids:
                    ad_unit = self.load_ad_unit(publisher_id, site_id, unit_id)
                    if ad_unit:
                        ad_units.append(ad_unit)
                return ad_units

            if not self._ensure_connected():
                return ad_units

            with self._conn.cursor() as cur:
                cur.execute(
                    "SELECT unit_id FROM config_ad_units WHERE publisher_id = %s AND site_id = %s ORDER BY name",
                    (publisher_id, site_id),
                )
                unit_ids = [row[0] for row in cur.fetchall()]

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
        global_config: FeatureConfig | None = None,
        publishers: dict[str, PublisherConfigV2] | None = None,
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
                pub_id: config.to_dict() for pub_id, config in publishers.items()
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
        for _pub_id, pub_data in data.get("publishers", {}).items():
            config = _parse_publisher_from_dict(pub_data)
            if not self.save_publisher(config):
                success = False

        return success

    def clear_all(self) -> bool:
        """Clear all configurations (use with caution!)."""
        try:
            if self._use_memory:
                self._memory_store.clear()
                return True

            if not self._ensure_connected():
                return False

            with self._conn.cursor() as cur:
                # Delete in order to respect foreign keys
                cur.execute("DELETE FROM config_ad_units")
                cur.execute("DELETE FROM config_sites")
                cur.execute("DELETE FROM config_publishers")
                cur.execute("DELETE FROM config_global")
            self._conn.commit()

            logger.warning("All configurations cleared")
            return True
        except Exception as e:
            logger.error(f"Failed to clear configurations: {e}")
            if self._conn:
                self._conn.rollback()
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
_store: ConfigStore | None = None


def get_config_store() -> ConfigStore:
    """Get the global config store instance."""
    global _store
    if _store is None:
        _store = ConfigStore()
    return _store


def init_config_store(
    host: str | None = None,
    port: int | None = None,
    database: str | None = None,
    user: str | None = None,
    password: str | None = None,
    database_url: str | None = None,
) -> ConfigStore:
    """Initialize the global config store with specific connection parameters."""
    global _store
    _store = ConfigStore(
        host=host,
        port=port,
        database=database,
        user=user,
        password=password,
        database_url=database_url,
    )
    return _store
