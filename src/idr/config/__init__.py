"""
IDR Configuration Module

Provides publisher-specific configuration management for multi-tenant deployments.
"""

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

__all__ = [
    "PublisherConfig",
    "PublisherConfigManager",
    "BidderConfig",
    "SiteConfig",
    "IDRConfig",
    "RateLimitConfig",
    "PrivacyConfig",
    "get_publisher_config",
    "get_publisher_config_manager",
]
