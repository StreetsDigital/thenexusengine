"""
IDR Configuration Module

Provides publisher-specific configuration management for multi-tenant deployments.

Supports hierarchical configuration with inheritance:
    Global → Publisher → Site → Ad Unit

Key components:
    - FeatureConfig: Comprehensive configuration for any level
    - ConfigResolver: Resolves effective config through inheritance
    - resolve_config(): Convenience function for config resolution
"""

# Legacy publisher config (for backwards compatibility)
from .publisher_config import (
    BidderConfig,
    IDRConfig,
    PrivacyConfig,
    PublisherConfig,
    PublisherConfigManager,
    RateLimitConfig,
    SiteConfig,
    get_publisher_config,
    get_publisher_config_manager,
)

# New hierarchical configuration system
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

from .config_resolver import (
    AdUnitConfig,
    ConfigResolver,
    PublisherConfigV2,
    SiteConfig as SiteConfigV2,
    get_config_resolver,
    merge_configs,
    resolve_config,
    to_resolved_config,
)

from .config_store import (
    ConfigStore,
    get_config_store,
    init_config_store,
)

__all__ = [
    # Legacy (v1)
    "PublisherConfig",
    "PublisherConfigManager",
    "BidderConfig",
    "SiteConfig",
    "IDRConfig",
    "RateLimitConfig",
    "PrivacyConfig",
    "get_publisher_config",
    "get_publisher_config_manager",

    # Hierarchical config (v2)
    "ConfigLevel",
    "FeatureConfig",
    "IDRSettings",
    "PrivacySettings",
    "BidderSettings",
    "FloorSettings",
    "RateLimitSettings",
    "FeatureFlags",
    "ResolvedConfig",
    "get_default_global_config",

    # Config resolution
    "ConfigResolver",
    "PublisherConfigV2",
    "SiteConfigV2",
    "AdUnitConfig",
    "get_config_resolver",
    "resolve_config",
    "merge_configs",
    "to_resolved_config",

    # Config persistence (PostgreSQL)
    "ConfigStore",
    "get_config_store",
    "init_config_store",
]
