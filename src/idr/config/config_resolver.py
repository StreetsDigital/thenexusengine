"""
Configuration Resolver

Resolves the effective configuration for any entity by walking up the
inheritance chain: Ad Unit → Site → Publisher → Global.

Child configurations inherit from parent and can override specific values.
"""

import json
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Optional

import yaml

from .feature_config import (
    BidderSettings,
    ConfigLevel,
    FeatureConfig,
    FeatureFlags,
    FloorSettings,
    IDRSettings,
    PrivacySettings,
    RateLimitSettings,
    ResolvedConfig,
    get_default_global_config,
)


def _merge_value(child_value: Any, parent_value: Any) -> Any:
    """
    Merge child value with parent value.

    If child is None, use parent. Otherwise use child.
    For lists, child replaces parent (no merging).
    For dicts, deep merge.
    """
    if child_value is None:
        return parent_value
    if isinstance(child_value, dict) and isinstance(parent_value, dict):
        result = parent_value.copy()
        result.update(child_value)
        return result
    return child_value


def _merge_settings(child: Any, parent: Any) -> Any:
    """
    Merge settings dataclass, preferring non-None child values.
    """
    if child is None:
        return parent
    if parent is None:
        return child

    # Both are settings objects
    merged_dict = {}
    for key in child.__dict__:
        child_val = getattr(child, key)
        parent_val = getattr(parent, key)
        merged_dict[key] = _merge_value(child_val, parent_val)

    return type(child)(**merged_dict)


def merge_configs(child: FeatureConfig, parent: FeatureConfig) -> FeatureConfig:
    """
    Merge child config with parent config.

    Child values override parent values. None values in child
    inherit from parent.
    """
    return FeatureConfig(
        config_id=child.config_id,
        config_level=child.config_level,
        parent_id=child.parent_id,
        name=child.name,
        description=child.description,
        enabled=child.enabled,
        created_at=child.created_at,
        updated_at=child.updated_at,
        idr=_merge_settings(child.idr, parent.idr),
        privacy=_merge_settings(child.privacy, parent.privacy),
        bidders=_merge_settings(child.bidders, parent.bidders),
        floors=_merge_settings(child.floors, parent.floors),
        rate_limits=_merge_settings(child.rate_limits, parent.rate_limits),
        features=_merge_settings(child.features, parent.features),
        tags=child.tags or parent.tags,
        metadata={**parent.metadata, **child.metadata},
    )


def to_resolved_config(config: FeatureConfig, resolution_chain: list[str]) -> ResolvedConfig:
    """
    Convert a merged FeatureConfig to a fully resolved config.

    All None values become their defaults.
    """
    default = get_default_global_config()

    # Helper to get value or default
    def get_val(settings: Any, key: str, default_settings: Any) -> Any:
        val = getattr(settings, key, None) if settings else None
        if val is None:
            return getattr(default_settings, key, None)
        return val

    idr = config.idr or default.idr
    privacy = config.privacy or default.privacy
    bidders = config.bidders or default.bidders
    floors = config.floors or default.floors
    rate_limits = config.rate_limits or default.rate_limits
    features = config.features or default.features

    return ResolvedConfig(
        config_id=config.config_id,
        config_level=config.config_level,
        resolution_chain=resolution_chain,

        # IDR settings
        idr_enabled=get_val(idr, 'enabled', default.idr) or True,
        bypass_enabled=get_val(idr, 'bypass_enabled', default.idr) or False,
        shadow_mode=get_val(idr, 'shadow_mode', default.idr) or False,
        max_bidders=get_val(idr, 'max_bidders', default.idr) or 15,
        min_score_threshold=get_val(idr, 'min_score_threshold', default.idr) or 25.0,
        exploration_enabled=get_val(idr, 'exploration_enabled', default.idr) if get_val(idr, 'exploration_enabled', default.idr) is not None else True,
        exploration_rate=get_val(idr, 'exploration_rate', default.idr) or 0.1,
        exploration_slots=get_val(idr, 'exploration_slots', default.idr) or 2,
        low_confidence_threshold=get_val(idr, 'low_confidence_threshold', default.idr) or 0.5,
        exploration_confidence_threshold=get_val(idr, 'exploration_confidence_threshold', default.idr) or 0.3,
        anchor_bidders_enabled=get_val(idr, 'anchor_bidders_enabled', default.idr) if get_val(idr, 'anchor_bidders_enabled', default.idr) is not None else True,
        anchor_bidder_count=get_val(idr, 'anchor_bidder_count', default.idr) or 3,
        custom_anchor_bidders=get_val(idr, 'custom_anchor_bidders', default.idr) or [],
        diversity_enabled=get_val(idr, 'diversity_enabled', default.idr) if get_val(idr, 'diversity_enabled', default.idr) is not None else True,
        diversity_categories=get_val(idr, 'diversity_categories', default.idr) or ['premium', 'mid_tier', 'video_specialist', 'native'],
        scoring_weights=get_val(idr, 'scoring_weights', default.idr) or {
            'win_rate': 0.25, 'bid_rate': 0.20, 'cpm': 0.15,
            'floor_clearance': 0.15, 'latency': 0.10, 'recency': 0.10, 'id_match': 0.05,
        },
        latency_excellent_ms=get_val(idr, 'latency_excellent_ms', default.idr) or 100,
        latency_poor_ms=get_val(idr, 'latency_poor_ms', default.idr) or 500,
        selection_timeout_ms=get_val(idr, 'selection_timeout_ms', default.idr) or 50,

        # Privacy settings
        privacy_enabled=get_val(privacy, 'privacy_enabled', default.privacy) if get_val(privacy, 'privacy_enabled', default.privacy) is not None else True,
        privacy_strict_mode=get_val(privacy, 'privacy_strict_mode', default.privacy) or False,
        gdpr_applies=get_val(privacy, 'gdpr_applies', default.privacy) if get_val(privacy, 'gdpr_applies', default.privacy) is not None else True,
        ccpa_applies=get_val(privacy, 'ccpa_applies', default.privacy) if get_val(privacy, 'ccpa_applies', default.privacy) is not None else True,
        coppa_applies=get_val(privacy, 'coppa_applies', default.privacy) or False,
        require_consent=get_val(privacy, 'require_consent', default.privacy) if get_val(privacy, 'require_consent', default.privacy) is not None else True,
        allow_legitimate_interest=get_val(privacy, 'allow_legitimate_interest', default.privacy) if get_val(privacy, 'allow_legitimate_interest', default.privacy) is not None else True,

        # Bidder settings
        enabled_bidders=get_val(bidders, 'enabled_bidders', default.bidders) or [],
        disabled_bidders=get_val(bidders, 'disabled_bidders', default.bidders) or [],
        bidder_params=get_val(bidders, 'bidder_params', default.bidders) or {},
        bidder_allowlist=get_val(bidders, 'bidder_allowlist', default.bidders) or [],
        bidder_blocklist=get_val(bidders, 'bidder_blocklist', default.bidders) or [],

        # Floor settings
        floor_enabled=get_val(floors, 'floor_enabled', default.floors) if get_val(floors, 'floor_enabled', default.floors) is not None else True,
        default_floor_price=get_val(floors, 'default_floor_price', default.floors) or 0.0,
        floor_currency=get_val(floors, 'floor_currency', default.floors) or "USD",
        dynamic_floors_enabled=get_val(floors, 'dynamic_floors_enabled', default.floors) or False,
        floor_adjustment_factor=get_val(floors, 'floor_adjustment_factor', default.floors) or 1.0,
        banner_floor=get_val(floors, 'banner_floor', default.floors) or 0.0,
        video_floor=get_val(floors, 'video_floor', default.floors) or 0.0,
        native_floor=get_val(floors, 'native_floor', default.floors) or 0.0,
        audio_floor=get_val(floors, 'audio_floor', default.floors) or 0.0,

        # Rate limit settings
        rate_limit_enabled=get_val(rate_limits, 'rate_limit_enabled', default.rate_limits) if get_val(rate_limits, 'rate_limit_enabled', default.rate_limits) is not None else True,
        requests_per_second=get_val(rate_limits, 'requests_per_second', default.rate_limits) or 1000,
        burst=get_val(rate_limits, 'burst', default.rate_limits) or 100,

        # Feature flags
        prebid_enabled=get_val(features, 'prebid_enabled', default.features) if get_val(features, 'prebid_enabled', default.features) is not None else True,
        header_bidding_enabled=get_val(features, 'header_bidding_enabled', default.features) if get_val(features, 'header_bidding_enabled', default.features) is not None else True,
        lazy_loading_enabled=get_val(features, 'lazy_loading_enabled', default.features) or False,
        refresh_enabled=get_val(features, 'refresh_enabled', default.features) or False,
        refresh_interval_seconds=get_val(features, 'refresh_interval_seconds', default.features) or 30,
        analytics_enabled=get_val(features, 'analytics_enabled', default.features) if get_val(features, 'analytics_enabled', default.features) is not None else True,
        detailed_logging_enabled=get_val(features, 'detailed_logging_enabled', default.features) or False,
        ab_testing_enabled=get_val(features, 'ab_testing_enabled', default.features) or False,
        ab_test_group=get_val(features, 'ab_test_group', default.features) or "",
        custom_features=get_val(features, 'custom_features', default.features) or {},
    )


@dataclass
class AdUnitConfig:
    """Configuration for an ad unit."""
    unit_id: str
    name: str = ""
    enabled: bool = True
    sizes: list[list[int]] = field(default_factory=list)
    media_type: str = "banner"
    position: str = "unknown"
    floor_price: Optional[float] = None
    floor_currency: str = "USD"
    video: Optional[dict[str, Any]] = None

    # Ad unit specific feature overrides
    features: FeatureConfig = field(default_factory=FeatureConfig)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        result = {
            "unit_id": self.unit_id,
            "name": self.name,
            "enabled": self.enabled,
            "sizes": self.sizes,
            "media_type": self.media_type,
            "position": self.position,
            "floor_price": self.floor_price,
            "floor_currency": self.floor_currency,
        }
        if self.video:
            result["video"] = self.video
        if self.features.config_id:
            result["features"] = self.features.to_dict()
        return result


@dataclass
class SiteConfig:
    """Configuration for a site."""
    site_id: str
    domain: str
    name: str = ""
    enabled: bool = True
    ad_units: list[AdUnitConfig] = field(default_factory=list)

    # Site specific feature overrides
    features: FeatureConfig = field(default_factory=FeatureConfig)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        result = {
            "site_id": self.site_id,
            "domain": self.domain,
            "name": self.name,
            "enabled": self.enabled,
            "ad_units": [u.to_dict() for u in self.ad_units],
        }
        if self.features.config_id:
            result["features"] = self.features.to_dict()
        return result


@dataclass
class PublisherConfigV2:
    """
    Enhanced publisher configuration with hierarchical feature support.

    Supports per-publisher, per-site, and per-ad-unit configuration.
    """
    publisher_id: str
    name: str
    enabled: bool = True
    contact_email: str = ""
    contact_name: str = ""

    # Sites
    sites: list[SiteConfig] = field(default_factory=list)

    # Legacy bidder config (for backwards compatibility)
    bidders: dict[str, Any] = field(default_factory=dict)

    # API key
    api_key: str = ""
    api_key_enabled: bool = True

    # Publisher-level feature configuration
    features: FeatureConfig = field(default_factory=FeatureConfig)

    def get_site(self, site_id: str) -> Optional[SiteConfig]:
        """Get a site by ID."""
        for site in self.sites:
            if site.site_id == site_id:
                return site
        return None

    def get_ad_unit(self, site_id: str, unit_id: str) -> Optional[AdUnitConfig]:
        """Get an ad unit by site and unit ID."""
        site = self.get_site(site_id)
        if site:
            for unit in site.ad_units:
                if unit.unit_id == unit_id:
                    return unit
        return None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        return {
            "publisher_id": self.publisher_id,
            "name": self.name,
            "enabled": self.enabled,
            "contact_email": self.contact_email,
            "contact_name": self.contact_name,
            "sites": [s.to_dict() for s in self.sites],
            "bidders": self.bidders,
            "api_key": self.api_key,
            "api_key_enabled": self.api_key_enabled,
            "features": self.features.to_dict(),
        }


class ConfigResolver:
    """
    Resolves effective configuration for any entity.

    Walks the inheritance chain: Ad Unit → Site → Publisher → Global

    Supports Redis persistence for configurations - all changes are
    automatically saved to Redis when persist=True (default).
    """

    def __init__(
        self,
        global_config: Optional[FeatureConfig] = None,
        config_dir: Optional[str] = None,
        persist: bool = True,
        auto_load: bool = True,
    ):
        """
        Initialize the config resolver.

        Args:
            global_config: Global default configuration
            config_dir: Directory containing publisher config files
            persist: Whether to persist changes to Redis (default: True)
            auto_load: Whether to load existing configs from Redis on init (default: True)
        """
        self._global_config = global_config or get_default_global_config()
        self._config_dir = Path(config_dir) if config_dir else None
        self._persist = persist

        # Caches
        self._publisher_configs: dict[str, PublisherConfigV2] = {}
        self._resolved_cache: dict[str, ResolvedConfig] = {}

        # Config store for persistence
        self._store: Optional[Any] = None

        # Auto-load from Redis if enabled
        if auto_load and persist:
            self._load_from_store()

    def _get_store(self) -> Any:
        """Get or create the config store."""
        if self._store is None:
            from .config_store import get_config_store
            self._store = get_config_store()
        return self._store

    def _load_from_store(self) -> None:
        """Load all configurations from Redis."""
        try:
            store = self._get_store()
            self._global_config = store.load_global_config()
            self._publisher_configs = store.load_all_publishers()
        except Exception as e:
            import logging
            logging.warning(f"Failed to load configs from store: {e}")

    def set_global_config(self, config: FeatureConfig, save: bool = True) -> None:
        """
        Set the global configuration.

        Args:
            config: The new global configuration
            save: Whether to persist to Redis (default: True)
        """
        self._global_config = config
        self._resolved_cache.clear()

        if save and self._persist:
            self._get_store().save_global_config(config)

    def get_global_config(self) -> FeatureConfig:
        """Get the global configuration."""
        return self._global_config

    def register_publisher(self, config: PublisherConfigV2, save: bool = True) -> None:
        """
        Register a publisher configuration.

        Args:
            config: The publisher configuration
            save: Whether to persist to Redis (default: True)
        """
        self._publisher_configs[config.publisher_id] = config
        # Clear cache for this publisher
        self._clear_publisher_cache(config.publisher_id)

        if save and self._persist:
            self._get_store().save_publisher(config)

    def delete_publisher(self, publisher_id: str, save: bool = True) -> bool:
        """
        Delete a publisher configuration.

        Args:
            publisher_id: The publisher ID to delete
            save: Whether to persist deletion to Redis (default: True)

        Returns:
            True if deleted, False if not found
        """
        if publisher_id not in self._publisher_configs:
            return False

        del self._publisher_configs[publisher_id]
        self._clear_publisher_cache(publisher_id)

        if save and self._persist:
            self._get_store().delete_publisher(publisher_id)

        return True

    def save_site(self, publisher_id: str, site: SiteConfig, save: bool = True) -> bool:
        """
        Save a site configuration.

        Args:
            publisher_id: The publisher ID
            site: The site configuration
            save: Whether to persist to Redis (default: True)

        Returns:
            True if saved successfully
        """
        publisher = self._publisher_configs.get(publisher_id)
        if not publisher:
            return False

        # Update or add site
        site_found = False
        for i, s in enumerate(publisher.sites):
            if s.site_id == site.site_id:
                publisher.sites[i] = site
                site_found = True
                break

        if not site_found:
            publisher.sites.append(site)

        self._clear_publisher_cache(publisher_id)

        if save and self._persist:
            self._get_store().save_site(publisher_id, site)

        return True

    def delete_site(self, publisher_id: str, site_id: str, save: bool = True) -> bool:
        """Delete a site configuration."""
        publisher = self._publisher_configs.get(publisher_id)
        if not publisher:
            return False

        publisher.sites = [s for s in publisher.sites if s.site_id != site_id]
        self._clear_publisher_cache(publisher_id)

        if save and self._persist:
            self._get_store().delete_site(publisher_id, site_id)

        return True

    def save_ad_unit(
        self, publisher_id: str, site_id: str, ad_unit: AdUnitConfig, save: bool = True
    ) -> bool:
        """
        Save an ad unit configuration.

        Args:
            publisher_id: The publisher ID
            site_id: The site ID
            ad_unit: The ad unit configuration
            save: Whether to persist to Redis (default: True)

        Returns:
            True if saved successfully
        """
        publisher = self._publisher_configs.get(publisher_id)
        if not publisher:
            return False

        site = publisher.get_site(site_id)
        if not site:
            return False

        # Update or add ad unit
        unit_found = False
        for i, u in enumerate(site.ad_units):
            if u.unit_id == ad_unit.unit_id:
                site.ad_units[i] = ad_unit
                unit_found = True
                break

        if not unit_found:
            site.ad_units.append(ad_unit)

        self._clear_publisher_cache(publisher_id)

        if save and self._persist:
            self._get_store().save_ad_unit(publisher_id, site_id, ad_unit)

        return True

    def delete_ad_unit(
        self, publisher_id: str, site_id: str, unit_id: str, save: bool = True
    ) -> bool:
        """Delete an ad unit configuration."""
        publisher = self._publisher_configs.get(publisher_id)
        if not publisher:
            return False

        site = publisher.get_site(site_id)
        if not site:
            return False

        site.ad_units = [u for u in site.ad_units if u.unit_id != unit_id]
        self._clear_publisher_cache(publisher_id)

        if save and self._persist:
            self._get_store().delete_ad_unit(publisher_id, site_id, unit_id)

        return True

    def sync_to_store(self) -> bool:
        """
        Sync all in-memory configurations to Redis.

        Returns:
            True if sync successful
        """
        if not self._persist:
            return False

        store = self._get_store()
        return store.save_all(self._global_config, self._publisher_configs)

    def sync_from_store(self) -> bool:
        """
        Reload all configurations from Redis.

        Returns:
            True if sync successful
        """
        try:
            self._load_from_store()
            self._resolved_cache.clear()
            return True
        except Exception:
            return False

    def export_configs(self) -> dict[str, Any]:
        """Export all configurations to a dictionary."""
        return {
            "global": self._global_config.to_dict(),
            "publishers": {
                pub_id: config.to_dict()
                for pub_id, config in self._publisher_configs.items()
            },
        }

    def import_configs(self, data: dict[str, Any], save: bool = True) -> bool:
        """
        Import configurations from a dictionary.

        Args:
            data: Configuration data with 'global' and 'publishers' keys
            save: Whether to persist to Redis (default: True)

        Returns:
            True if import successful
        """
        try:
            if "global" in data:
                self._global_config = FeatureConfig.from_dict(data["global"])

            for pub_id, pub_data in data.get("publishers", {}).items():
                config = self._parse_publisher_config(pub_data)
                self._publisher_configs[pub_id] = config

            self._resolved_cache.clear()

            if save and self._persist:
                self._get_store().save_all(self._global_config, self._publisher_configs)

            return True
        except Exception:
            return False

    def get_publisher_config(self, publisher_id: str) -> Optional[PublisherConfigV2]:
        """Get a publisher configuration."""
        return self._publisher_configs.get(publisher_id)

    def resolve_for_publisher(self, publisher_id: str) -> ResolvedConfig:
        """
        Resolve effective configuration for a publisher.

        Chain: Publisher → Global
        """
        cache_key = f"publisher:{publisher_id}"
        if cache_key in self._resolved_cache:
            return self._resolved_cache[cache_key]

        resolution_chain = ["global"]
        merged = self._global_config

        publisher = self._publisher_configs.get(publisher_id)
        if publisher and publisher.features.config_id:
            merged = merge_configs(publisher.features, merged)
            resolution_chain.append(f"publisher:{publisher_id}")

        resolved = to_resolved_config(merged, resolution_chain)
        resolved.config_id = cache_key
        resolved.config_level = ConfigLevel.PUBLISHER

        self._resolved_cache[cache_key] = resolved
        return resolved

    def resolve_for_site(self, publisher_id: str, site_id: str) -> ResolvedConfig:
        """
        Resolve effective configuration for a site.

        Chain: Site → Publisher → Global
        """
        cache_key = f"site:{publisher_id}:{site_id}"
        if cache_key in self._resolved_cache:
            return self._resolved_cache[cache_key]

        resolution_chain = ["global"]
        merged = self._global_config

        publisher = self._publisher_configs.get(publisher_id)
        if publisher:
            if publisher.features.config_id:
                merged = merge_configs(publisher.features, merged)
                resolution_chain.append(f"publisher:{publisher_id}")

            site = publisher.get_site(site_id)
            if site and site.features.config_id:
                merged = merge_configs(site.features, merged)
                resolution_chain.append(f"site:{site_id}")

        resolved = to_resolved_config(merged, resolution_chain)
        resolved.config_id = cache_key
        resolved.config_level = ConfigLevel.SITE

        self._resolved_cache[cache_key] = resolved
        return resolved

    def resolve_for_ad_unit(
        self, publisher_id: str, site_id: str, unit_id: str
    ) -> ResolvedConfig:
        """
        Resolve effective configuration for an ad unit.

        Chain: Ad Unit → Site → Publisher → Global
        """
        cache_key = f"ad_unit:{publisher_id}:{site_id}:{unit_id}"
        if cache_key in self._resolved_cache:
            return self._resolved_cache[cache_key]

        resolution_chain = ["global"]
        merged = self._global_config

        publisher = self._publisher_configs.get(publisher_id)
        if publisher:
            if publisher.features.config_id:
                merged = merge_configs(publisher.features, merged)
                resolution_chain.append(f"publisher:{publisher_id}")

            site = publisher.get_site(site_id)
            if site:
                if site.features.config_id:
                    merged = merge_configs(site.features, merged)
                    resolution_chain.append(f"site:{site_id}")

                ad_unit = None
                for unit in site.ad_units:
                    if unit.unit_id == unit_id:
                        ad_unit = unit
                        break

                if ad_unit and ad_unit.features.config_id:
                    merged = merge_configs(ad_unit.features, merged)
                    resolution_chain.append(f"ad_unit:{unit_id}")

        resolved = to_resolved_config(merged, resolution_chain)
        resolved.config_id = cache_key
        resolved.config_level = ConfigLevel.AD_UNIT

        self._resolved_cache[cache_key] = resolved
        return resolved

    def resolve(
        self,
        publisher_id: Optional[str] = None,
        site_id: Optional[str] = None,
        unit_id: Optional[str] = None,
    ) -> ResolvedConfig:
        """
        Resolve configuration for any level.

        Automatically determines the appropriate resolution method.
        """
        if unit_id and site_id and publisher_id:
            return self.resolve_for_ad_unit(publisher_id, site_id, unit_id)
        elif site_id and publisher_id:
            return self.resolve_for_site(publisher_id, site_id)
        elif publisher_id:
            return self.resolve_for_publisher(publisher_id)
        else:
            return to_resolved_config(self._global_config, ["global"])

    def _clear_publisher_cache(self, publisher_id: str) -> None:
        """Clear cache entries for a publisher and its children."""
        keys_to_remove = [
            k for k in self._resolved_cache
            if publisher_id in k
        ]
        for key in keys_to_remove:
            del self._resolved_cache[key]

    def clear_cache(self) -> None:
        """Clear all cached resolutions."""
        self._resolved_cache.clear()

    def load_from_yaml(self, path: Path) -> PublisherConfigV2:
        """
        Load a publisher configuration from YAML file.
        """
        with open(path) as f:
            data = yaml.safe_load(f)

        return self._parse_publisher_config(data)

    def _parse_publisher_config(self, data: dict[str, Any]) -> PublisherConfigV2:
        """Parse publisher config from dictionary."""
        # Parse contact
        contact = data.get("contact", {})

        # Parse sites
        sites = []
        for site_data in data.get("sites", []):
            site = self._parse_site_config(site_data)
            sites.append(site)

        # Parse publisher-level features
        features = FeatureConfig()
        if "features" in data:
            features = FeatureConfig.from_dict(data["features"])
            features.config_id = f"publisher:{data.get('publisher_id', '')}"
            features.config_level = ConfigLevel.PUBLISHER

        # Handle legacy idr config
        if "idr" in data and not features.idr.enabled:
            idr_data = data["idr"]
            features.idr = IDRSettings(
                enabled=True,
                max_bidders=idr_data.get("max_bidders"),
                min_score_threshold=idr_data.get("min_score"),
                selection_timeout_ms=idr_data.get("timeout_ms"),
            )
            features.config_id = f"publisher:{data.get('publisher_id', '')}"

        # Handle legacy privacy config
        if "privacy" in data:
            priv_data = data["privacy"]
            features.privacy = PrivacySettings(
                gdpr_applies=priv_data.get("gdpr_applies"),
                ccpa_applies=priv_data.get("ccpa_applies"),
                coppa_applies=priv_data.get("coppa_applies"),
            )

        # Handle legacy rate_limits
        if "rate_limits" in data:
            rl_data = data["rate_limits"]
            features.rate_limits = RateLimitSettings(
                requests_per_second=rl_data.get("requests_per_second"),
                burst=rl_data.get("burst"),
            )

        # API key
        api_key_data = data.get("api_key", {})
        api_key = api_key_data.get("key", "") if isinstance(api_key_data, dict) else ""

        return PublisherConfigV2(
            publisher_id=data.get("publisher_id", ""),
            name=data.get("name", ""),
            enabled=data.get("enabled", True),
            contact_email=contact.get("email", ""),
            contact_name=contact.get("name", ""),
            sites=sites,
            bidders=data.get("bidders", {}),
            api_key=api_key,
            features=features,
        )

    def _parse_site_config(self, data: dict[str, Any]) -> SiteConfig:
        """Parse site config from dictionary."""
        # Parse ad units
        ad_units = []
        for unit_data in data.get("ad_units", []):
            ad_unit = self._parse_ad_unit_config(unit_data)
            ad_units.append(ad_unit)

        # Parse site-level features
        features = FeatureConfig()
        if "features" in data:
            features = FeatureConfig.from_dict(data["features"])
            features.config_id = f"site:{data.get('site_id', '')}"
            features.config_level = ConfigLevel.SITE

        return SiteConfig(
            site_id=data.get("site_id", ""),
            domain=data.get("domain", ""),
            name=data.get("name", ""),
            enabled=data.get("enabled", True),
            ad_units=ad_units,
            features=features,
        )

    def _parse_ad_unit_config(self, data: dict[str, Any]) -> AdUnitConfig:
        """Parse ad unit config from dictionary."""
        # Parse ad unit-level features
        features = FeatureConfig()
        if "features" in data:
            features = FeatureConfig.from_dict(data["features"])
            features.config_id = f"ad_unit:{data.get('unit_id', '')}"
            features.config_level = ConfigLevel.AD_UNIT

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

    def save_publisher_config(self, config: PublisherConfigV2, path: Path) -> None:
        """Save a publisher configuration to YAML file."""
        data = self._serialize_publisher_config(config)
        with open(path, "w") as f:
            yaml.dump(data, f, default_flow_style=False, sort_keys=False)

    def _serialize_publisher_config(self, config: PublisherConfigV2) -> dict[str, Any]:
        """Serialize publisher config to dictionary for YAML."""
        data = {
            "publisher_id": config.publisher_id,
            "name": config.name,
            "enabled": config.enabled,
        }

        if config.contact_email or config.contact_name:
            data["contact"] = {
                "email": config.contact_email,
                "name": config.contact_name,
            }

        # Sites
        if config.sites:
            data["sites"] = [self._serialize_site_config(s) for s in config.sites]

        # Bidders (legacy)
        if config.bidders:
            data["bidders"] = config.bidders

        # Features
        if config.features.config_id:
            data["features"] = config.features.to_dict()

        return data

    def _serialize_site_config(self, config: SiteConfig) -> dict[str, Any]:
        """Serialize site config to dictionary."""
        data = {
            "site_id": config.site_id,
            "domain": config.domain,
            "name": config.name,
            "enabled": config.enabled,
        }

        if config.ad_units:
            data["ad_units"] = [self._serialize_ad_unit_config(u) for u in config.ad_units]

        if config.features.config_id:
            data["features"] = config.features.to_dict()

        return data

    def _serialize_ad_unit_config(self, config: AdUnitConfig) -> dict[str, Any]:
        """Serialize ad unit config to dictionary."""
        data = {
            "unit_id": config.unit_id,
            "name": config.name,
            "enabled": config.enabled,
            "sizes": config.sizes,
            "media_type": config.media_type,
            "position": config.position,
        }

        if config.floor_price is not None:
            data["floor_price"] = config.floor_price
            data["floor_currency"] = config.floor_currency

        if config.video:
            data["video"] = config.video

        if config.features.config_id:
            data["features"] = config.features.to_dict()

        return data


# Global resolver instance
_resolver: Optional[ConfigResolver] = None


def get_config_resolver() -> ConfigResolver:
    """Get the global config resolver instance."""
    global _resolver
    if _resolver is None:
        _resolver = ConfigResolver()
    return _resolver


def resolve_config(
    publisher_id: Optional[str] = None,
    site_id: Optional[str] = None,
    unit_id: Optional[str] = None,
) -> ResolvedConfig:
    """
    Convenience function to resolve configuration.

    Examples:
        # Get global config
        config = resolve_config()

        # Get publisher config
        config = resolve_config(publisher_id="pub123")

        # Get site config
        config = resolve_config(publisher_id="pub123", site_id="site456")

        # Get ad unit config
        config = resolve_config(publisher_id="pub123", site_id="site456", unit_id="unit789")
    """
    return get_config_resolver().resolve(publisher_id, site_id, unit_id)
