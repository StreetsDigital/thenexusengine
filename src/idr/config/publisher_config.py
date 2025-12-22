"""
Publisher Configuration Management

Loads and manages publisher-specific configurations for multi-tenant support.
Each publisher can have their own bidder credentials, rate limits, and IDR settings.
"""

import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml


@dataclass
class BidderConfig:
    """Configuration for a single bidder."""

    enabled: bool = True
    params: dict[str, Any] = field(default_factory=dict)


@dataclass
class SiteConfig:
    """Configuration for a publisher's site."""

    site_id: str
    domain: str
    name: str = ""


@dataclass
class IDRConfig:
    """IDR-specific settings for a publisher."""

    max_bidders: int = 8
    min_score: float = 0.1
    timeout_ms: int = 50


@dataclass
class RateLimitConfig:
    """Rate limiting configuration."""

    requests_per_second: int = 1000
    burst: int = 100


@dataclass
class PrivacyConfig:
    """Privacy settings for a publisher."""

    gdpr_applies: bool = True
    ccpa_applies: bool = True
    coppa_applies: bool = False


@dataclass
class APIKeyConfig:
    """API key configuration for PBS authentication."""

    key: str = ""
    created_at: str = ""
    last_used: str = ""
    enabled: bool = True


@dataclass
class RevenueShareConfig:
    """
    Revenue share configuration for a publisher.

    - platform_demand_rev_share: Percentage taken from platform-sourced demand partners.
      This is the margin on demand sources that the platform brings in.
      Applied on top of publisher's net earnings.
      Example: 15.0 means we take 15% of earnings from our demand partners.

    - publisher_own_demand_fee: Percentage fee charged when publishers use their own demand.
      This is billed monthly to the publisher.
      Example: 5.0 means we charge 5% on revenue from publisher's own demand sources.
    """
    platform_demand_rev_share: float = 0.0  # % taken from platform demand
    publisher_own_demand_fee: float = 0.0   # % charged for publisher's own demand

    def __post_init__(self):
        """Validate percentage values are within valid range (0-100)."""
        if not 0 <= self.platform_demand_rev_share <= 100:
            raise ValueError(
                f"platform_demand_rev_share must be between 0 and 100, got {self.platform_demand_rev_share}"
            )
        if not 0 <= self.publisher_own_demand_fee <= 100:
            raise ValueError(
                f"publisher_own_demand_fee must be between 0 and 100, got {self.publisher_own_demand_fee}"
            )


@dataclass
class PublisherConfig:
    """Complete configuration for a publisher."""

    publisher_id: str
    name: str
    enabled: bool = True
    contact_email: str = ""
    contact_name: str = ""
    sites: list[SiteConfig] = field(default_factory=list)
    bidders: dict[str, BidderConfig] = field(default_factory=dict)
    idr: IDRConfig = field(default_factory=IDRConfig)
    rate_limits: RateLimitConfig = field(default_factory=RateLimitConfig)
    privacy: PrivacyConfig = field(default_factory=PrivacyConfig)
    api_key: APIKeyConfig = field(default_factory=APIKeyConfig)
    revenue_share: RevenueShareConfig = field(default_factory=RevenueShareConfig)

    def get_enabled_bidders(self) -> list[str]:
        """Get list of enabled bidder codes."""
        return [code for code, config in self.bidders.items() if config.enabled]

    def get_bidder_params(self, bidder_code: str) -> dict[str, Any]:
        """Get parameters for a specific bidder."""
        if bidder_code in self.bidders:
            return self.bidders[bidder_code].params
        return {}


class PublisherConfigManager:
    """
    Manages publisher configurations from YAML files.

    Supports loading from:
    - Individual YAML files per publisher
    - Environment variable overrides
    - Default fallback configuration
    """

    def __init__(self, config_dir: str | None = None):
        """
        Initialize the config manager.

        Args:
            config_dir: Directory containing publisher YAML files.
                       Defaults to config/publishers/
        """
        if config_dir is None:
            # Default to config/publishers relative to project root
            config_dir = os.environ.get(
                "PUBLISHER_CONFIG_DIR",
                str(
                    Path(__file__).parent.parent.parent.parent / "config" / "publishers"
                ),
            )
        self.config_dir = Path(config_dir)
        self._cache: dict[str, PublisherConfig] = {}
        self._default_config: PublisherConfig | None = None

    def load_all(self) -> dict[str, PublisherConfig]:
        """Load all publisher configurations from the config directory."""
        configs = {}

        if not self.config_dir.exists():
            return configs

        for yaml_file in self.config_dir.glob("*.yaml"):
            if yaml_file.name == "example.yaml":
                continue  # Skip example file

            try:
                config = self._load_file(yaml_file)
                if config and config.enabled:
                    configs[config.publisher_id] = config
                    self._cache[config.publisher_id] = config
            except Exception as e:
                print(f"Error loading {yaml_file}: {e}")

        return configs

    def get(self, publisher_id: str) -> PublisherConfig | None:
        """
        Get configuration for a specific publisher.

        Args:
            publisher_id: The publisher's unique identifier

        Returns:
            PublisherConfig if found, None otherwise
        """
        # Check cache first
        if publisher_id in self._cache:
            return self._cache[publisher_id]

        # Try to load from file
        config_file = self.config_dir / f"{publisher_id}.yaml"
        if config_file.exists():
            config = self._load_file(config_file)
            if config:
                self._cache[publisher_id] = config
                return config

        # Return default config if available
        return self._default_config

    def get_or_default(self, publisher_id: str) -> PublisherConfig:
        """
        Get configuration for a publisher, or return default.

        Args:
            publisher_id: The publisher's unique identifier

        Returns:
            PublisherConfig (never None)
        """
        config = self.get(publisher_id)
        if config:
            return config

        # Create a default config for unknown publishers
        return PublisherConfig(
            publisher_id=publisher_id,
            name=f"Publisher {publisher_id}",
            enabled=True,
        )

    def set_default(self, config: PublisherConfig) -> None:
        """Set the default configuration for unknown publishers."""
        self._default_config = config

    def reload(self, publisher_id: str | None = None) -> None:
        """
        Reload configuration(s) from disk.

        Args:
            publisher_id: Specific publisher to reload, or None for all
        """
        if publisher_id:
            self._cache.pop(publisher_id, None)
            self.get(publisher_id)
        else:
            self._cache.clear()
            self.load_all()

    def _load_file(self, path: Path) -> PublisherConfig | None:
        """Load a single publisher config file."""
        try:
            with open(path) as f:
                data = yaml.safe_load(f)

            if not data:
                return None

            # Parse sites
            sites = []
            for site_data in data.get("sites", []):
                sites.append(
                    SiteConfig(
                        site_id=site_data.get("site_id", ""),
                        domain=site_data.get("domain", ""),
                        name=site_data.get("name", ""),
                    )
                )

            # Parse bidders
            bidders = {}
            for bidder_code, bidder_data in data.get("bidders", {}).items():
                bidders[bidder_code] = BidderConfig(
                    enabled=bidder_data.get("enabled", True),
                    params=bidder_data.get("params", {}),
                )

            # Parse IDR config
            idr_data = data.get("idr", {})
            idr_config = IDRConfig(
                max_bidders=idr_data.get("max_bidders", 8),
                min_score=idr_data.get("min_score", 0.1),
                timeout_ms=idr_data.get("timeout_ms", 50),
            )

            # Parse rate limits
            rate_data = data.get("rate_limits", {})
            rate_config = RateLimitConfig(
                requests_per_second=rate_data.get("requests_per_second", 1000),
                burst=rate_data.get("burst", 100),
            )

            # Parse privacy
            privacy_data = data.get("privacy", {})
            privacy_config = PrivacyConfig(
                gdpr_applies=privacy_data.get("gdpr_applies", True),
                ccpa_applies=privacy_data.get("ccpa_applies", True),
                coppa_applies=privacy_data.get("coppa_applies", False),
            )

            # Parse contact
            contact = data.get("contact", {})

            # Parse API key config
            api_key_data = data.get("api_key", {})
            api_key_config = APIKeyConfig(
                key=api_key_data.get("key", ""),
                created_at=api_key_data.get("created_at", ""),
                last_used=api_key_data.get("last_used", ""),
                enabled=api_key_data.get("enabled", True),
            )

            # Parse revenue share config
            revenue_share_data = data.get("revenue_share", {})
            revenue_share_config = RevenueShareConfig(
                platform_demand_rev_share=float(revenue_share_data.get("platform_demand_rev_share", 0.0)),
                publisher_own_demand_fee=float(revenue_share_data.get("publisher_own_demand_fee", 0.0)),
            )

            return PublisherConfig(
                publisher_id=data.get("publisher_id", path.stem),
                name=data.get("name", ""),
                enabled=data.get("enabled", True),
                contact_email=contact.get("email", ""),
                contact_name=contact.get("name", ""),
                sites=sites,
                bidders=bidders,
                idr=idr_config,
                rate_limits=rate_config,
                privacy=privacy_config,
                api_key=api_key_config,
                revenue_share=revenue_share_config,
            )

        except yaml.YAMLError as e:
            print(f"YAML error in {path}: {e}")
            return None


# Global instance for easy access
_manager: PublisherConfigManager | None = None


def get_publisher_config_manager() -> PublisherConfigManager:
    """Get the global publisher config manager instance."""
    global _manager
    if _manager is None:
        _manager = PublisherConfigManager()
        _manager.load_all()
    return _manager


def get_publisher_config(publisher_id: str) -> PublisherConfig:
    """
    Convenience function to get a publisher's configuration.

    Args:
        publisher_id: The publisher's unique identifier

    Returns:
        PublisherConfig for the publisher
    """
    return get_publisher_config_manager().get_or_default(publisher_id)
