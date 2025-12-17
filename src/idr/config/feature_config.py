"""
Hierarchical Feature Configuration System

Provides a comprehensive configuration model that supports inheritance:
Global → Publisher → Site → Ad Unit

Each level can override settings from its parent, allowing fine-grained control
over IDR behavior and feature flags at any level of the hierarchy.
"""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Optional
import copy


class ConfigLevel(str, Enum):
    """Configuration hierarchy levels."""
    GLOBAL = "global"
    PUBLISHER = "publisher"
    SITE = "site"
    AD_UNIT = "ad_unit"


@dataclass
class IDRSettings:
    """
    Comprehensive IDR settings that can be configured at any level.

    All fields are Optional to support partial overrides - None means
    "inherit from parent".
    """
    # Enable/Disable IDR
    enabled: Optional[bool] = None

    # Operating modes
    bypass_enabled: Optional[bool] = None      # Select ALL bidders (testing)
    shadow_mode: Optional[bool] = None         # Log decisions but don't filter

    # Selection limits
    max_bidders: Optional[int] = None          # Maximum partners per auction
    min_score_threshold: Optional[float] = None  # Minimum score to be considered

    # Exploration settings
    exploration_enabled: Optional[bool] = None
    exploration_rate: Optional[float] = None   # Probability to include low-confidence
    exploration_slots: Optional[int] = None    # Reserved slots for data gathering
    low_confidence_threshold: Optional[float] = None
    exploration_confidence_threshold: Optional[float] = None

    # Anchor bidders
    anchor_bidders_enabled: Optional[bool] = None
    anchor_bidder_count: Optional[int] = None  # Always-include top performers
    custom_anchor_bidders: Optional[list[str]] = None  # Override anchor bidders

    # Diversity settings
    diversity_enabled: Optional[bool] = None   # Enforce category diversity
    diversity_categories: Optional[list[str]] = None

    # Scoring weights (must sum to 1.0)
    scoring_weights: Optional[dict[str, float]] = None

    # Latency thresholds
    latency_excellent_ms: Optional[int] = None
    latency_poor_ms: Optional[int] = None

    # Timeout
    selection_timeout_ms: Optional[int] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary, excluding None values."""
        result = {}
        for key, value in self.__dict__.items():
            if value is not None:
                result[key] = value
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "IDRSettings":
        """Create from dictionary."""
        return cls(
            enabled=data.get("enabled"),
            bypass_enabled=data.get("bypass_enabled"),
            shadow_mode=data.get("shadow_mode"),
            max_bidders=data.get("max_bidders"),
            min_score_threshold=data.get("min_score_threshold"),
            exploration_enabled=data.get("exploration_enabled"),
            exploration_rate=data.get("exploration_rate"),
            exploration_slots=data.get("exploration_slots"),
            low_confidence_threshold=data.get("low_confidence_threshold"),
            exploration_confidence_threshold=data.get("exploration_confidence_threshold"),
            anchor_bidders_enabled=data.get("anchor_bidders_enabled"),
            anchor_bidder_count=data.get("anchor_bidder_count"),
            custom_anchor_bidders=data.get("custom_anchor_bidders"),
            diversity_enabled=data.get("diversity_enabled"),
            diversity_categories=data.get("diversity_categories"),
            scoring_weights=data.get("scoring_weights"),
            latency_excellent_ms=data.get("latency_excellent_ms"),
            latency_poor_ms=data.get("latency_poor_ms"),
            selection_timeout_ms=data.get("selection_timeout_ms"),
        )


@dataclass
class PrivacySettings:
    """
    Privacy settings configurable at any level.
    """
    # Privacy filter
    privacy_enabled: Optional[bool] = None
    privacy_strict_mode: Optional[bool] = None

    # Regulation applicability
    gdpr_applies: Optional[bool] = None
    ccpa_applies: Optional[bool] = None
    coppa_applies: Optional[bool] = None

    # Consent handling
    require_consent: Optional[bool] = None
    allow_legitimate_interest: Optional[bool] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary, excluding None values."""
        result = {}
        for key, value in self.__dict__.items():
            if value is not None:
                result[key] = value
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "PrivacySettings":
        """Create from dictionary."""
        return cls(
            privacy_enabled=data.get("privacy_enabled"),
            privacy_strict_mode=data.get("privacy_strict_mode"),
            gdpr_applies=data.get("gdpr_applies"),
            ccpa_applies=data.get("ccpa_applies"),
            coppa_applies=data.get("coppa_applies"),
            require_consent=data.get("require_consent"),
            allow_legitimate_interest=data.get("allow_legitimate_interest"),
        )


@dataclass
class BidderSettings:
    """
    Bidder-specific settings that can be configured at any level.
    """
    # Enable/disable specific bidders
    enabled_bidders: Optional[list[str]] = None
    disabled_bidders: Optional[list[str]] = None

    # Bidder-specific parameters (bidder_code -> params)
    bidder_params: Optional[dict[str, dict[str, Any]]] = None

    # Bidder allowlist/blocklist for this context
    bidder_allowlist: Optional[list[str]] = None
    bidder_blocklist: Optional[list[str]] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary, excluding None values."""
        result = {}
        for key, value in self.__dict__.items():
            if value is not None:
                result[key] = value
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "BidderSettings":
        """Create from dictionary."""
        return cls(
            enabled_bidders=data.get("enabled_bidders"),
            disabled_bidders=data.get("disabled_bidders"),
            bidder_params=data.get("bidder_params"),
            bidder_allowlist=data.get("bidder_allowlist"),
            bidder_blocklist=data.get("bidder_blocklist"),
        )


@dataclass
class FloorSettings:
    """
    Floor price settings configurable at any level.
    """
    # Floor prices
    floor_enabled: Optional[bool] = None
    default_floor_price: Optional[float] = None
    floor_currency: Optional[str] = None

    # Dynamic floors
    dynamic_floors_enabled: Optional[bool] = None
    floor_adjustment_factor: Optional[float] = None

    # Per-format floors
    banner_floor: Optional[float] = None
    video_floor: Optional[float] = None
    native_floor: Optional[float] = None
    audio_floor: Optional[float] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary, excluding None values."""
        result = {}
        for key, value in self.__dict__.items():
            if value is not None:
                result[key] = value
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "FloorSettings":
        """Create from dictionary."""
        return cls(
            floor_enabled=data.get("floor_enabled"),
            default_floor_price=data.get("default_floor_price"),
            floor_currency=data.get("floor_currency"),
            dynamic_floors_enabled=data.get("dynamic_floors_enabled"),
            floor_adjustment_factor=data.get("floor_adjustment_factor"),
            banner_floor=data.get("banner_floor"),
            video_floor=data.get("video_floor"),
            native_floor=data.get("native_floor"),
            audio_floor=data.get("audio_floor"),
        )


@dataclass
class RateLimitSettings:
    """
    Rate limiting settings configurable at any level.
    """
    rate_limit_enabled: Optional[bool] = None
    requests_per_second: Optional[int] = None
    burst: Optional[int] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary, excluding None values."""
        result = {}
        for key, value in self.__dict__.items():
            if value is not None:
                result[key] = value
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "RateLimitSettings":
        """Create from dictionary."""
        return cls(
            rate_limit_enabled=data.get("rate_limit_enabled"),
            requests_per_second=data.get("requests_per_second"),
            burst=data.get("burst"),
        )


@dataclass
class FeatureFlags:
    """
    Feature flags that can be toggled at any level.
    """
    # Core features
    prebid_enabled: Optional[bool] = None
    header_bidding_enabled: Optional[bool] = None

    # Optimization features
    lazy_loading_enabled: Optional[bool] = None
    refresh_enabled: Optional[bool] = None
    refresh_interval_seconds: Optional[int] = None

    # Analytics
    analytics_enabled: Optional[bool] = None
    detailed_logging_enabled: Optional[bool] = None

    # A/B testing
    ab_testing_enabled: Optional[bool] = None
    ab_test_group: Optional[str] = None

    # Custom features (extensible)
    custom_features: Optional[dict[str, bool]] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary, excluding None values."""
        result = {}
        for key, value in self.__dict__.items():
            if value is not None:
                result[key] = value
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "FeatureFlags":
        """Create from dictionary."""
        return cls(
            prebid_enabled=data.get("prebid_enabled"),
            header_bidding_enabled=data.get("header_bidding_enabled"),
            lazy_loading_enabled=data.get("lazy_loading_enabled"),
            refresh_enabled=data.get("refresh_enabled"),
            refresh_interval_seconds=data.get("refresh_interval_seconds"),
            analytics_enabled=data.get("analytics_enabled"),
            detailed_logging_enabled=data.get("detailed_logging_enabled"),
            ab_testing_enabled=data.get("ab_testing_enabled"),
            ab_test_group=data.get("ab_test_group"),
            custom_features=data.get("custom_features"),
        )


@dataclass
class FeatureConfig:
    """
    Complete feature configuration for any level (Global/Publisher/Site/Ad Unit).

    This is the main configuration object that contains all settings.
    All fields are optional to support partial overrides through inheritance.
    """
    # Metadata
    config_id: str = ""
    config_level: ConfigLevel = ConfigLevel.GLOBAL
    parent_id: Optional[str] = None  # ID of parent config (for inheritance)
    name: str = ""
    description: str = ""
    enabled: bool = True

    # Timestamps
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None

    # Settings groups
    idr: IDRSettings = field(default_factory=IDRSettings)
    privacy: PrivacySettings = field(default_factory=PrivacySettings)
    bidders: BidderSettings = field(default_factory=BidderSettings)
    floors: FloorSettings = field(default_factory=FloorSettings)
    rate_limits: RateLimitSettings = field(default_factory=RateLimitSettings)
    features: FeatureFlags = field(default_factory=FeatureFlags)

    # Custom metadata
    tags: list[str] = field(default_factory=list)
    metadata: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "config_id": self.config_id,
            "config_level": self.config_level.value,
            "parent_id": self.parent_id,
            "name": self.name,
            "description": self.description,
            "enabled": self.enabled,
            "created_at": self.created_at.isoformat() if self.created_at else None,
            "updated_at": self.updated_at.isoformat() if self.updated_at else None,
            "idr": self.idr.to_dict(),
            "privacy": self.privacy.to_dict(),
            "bidders": self.bidders.to_dict(),
            "floors": self.floors.to_dict(),
            "rate_limits": self.rate_limits.to_dict(),
            "features": self.features.to_dict(),
            "tags": self.tags,
            "metadata": self.metadata,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "FeatureConfig":
        """Create from dictionary."""
        created_at = None
        updated_at = None

        if data.get("created_at"):
            created_at = datetime.fromisoformat(data["created_at"])
        if data.get("updated_at"):
            updated_at = datetime.fromisoformat(data["updated_at"])

        return cls(
            config_id=data.get("config_id", ""),
            config_level=ConfigLevel(data.get("config_level", "global")),
            parent_id=data.get("parent_id"),
            name=data.get("name", ""),
            description=data.get("description", ""),
            enabled=data.get("enabled", True),
            created_at=created_at,
            updated_at=updated_at,
            idr=IDRSettings.from_dict(data.get("idr", {})),
            privacy=PrivacySettings.from_dict(data.get("privacy", {})),
            bidders=BidderSettings.from_dict(data.get("bidders", {})),
            floors=FloorSettings.from_dict(data.get("floors", {})),
            rate_limits=RateLimitSettings.from_dict(data.get("rate_limits", {})),
            features=FeatureFlags.from_dict(data.get("features", {})),
            tags=data.get("tags", []),
            metadata=data.get("metadata", {}),
        )


@dataclass
class ResolvedConfig:
    """
    A fully resolved configuration with no None values.

    This represents the effective configuration after inheritance resolution.
    """
    # Metadata
    config_id: str
    config_level: ConfigLevel
    resolution_chain: list[str]  # IDs of configs used in resolution

    # Resolved IDR settings
    idr_enabled: bool = True
    bypass_enabled: bool = False
    shadow_mode: bool = False
    max_bidders: int = 15
    min_score_threshold: float = 25.0
    exploration_enabled: bool = True
    exploration_rate: float = 0.1
    exploration_slots: int = 2
    low_confidence_threshold: float = 0.5
    exploration_confidence_threshold: float = 0.3
    anchor_bidders_enabled: bool = True
    anchor_bidder_count: int = 3
    custom_anchor_bidders: list[str] = field(default_factory=list)
    diversity_enabled: bool = True
    diversity_categories: list[str] = field(
        default_factory=lambda: ['premium', 'mid_tier', 'video_specialist', 'native']
    )
    scoring_weights: dict[str, float] = field(default_factory=lambda: {
        'win_rate': 0.25,
        'bid_rate': 0.20,
        'cpm': 0.15,
        'floor_clearance': 0.15,
        'latency': 0.10,
        'recency': 0.10,
        'id_match': 0.05,
    })
    latency_excellent_ms: int = 100
    latency_poor_ms: int = 500
    selection_timeout_ms: int = 50

    # Resolved privacy settings
    privacy_enabled: bool = True
    privacy_strict_mode: bool = False
    gdpr_applies: bool = True
    ccpa_applies: bool = True
    coppa_applies: bool = False
    require_consent: bool = True
    allow_legitimate_interest: bool = True

    # Resolved bidder settings
    enabled_bidders: list[str] = field(default_factory=list)
    disabled_bidders: list[str] = field(default_factory=list)
    bidder_params: dict[str, dict[str, Any]] = field(default_factory=dict)
    bidder_allowlist: list[str] = field(default_factory=list)
    bidder_blocklist: list[str] = field(default_factory=list)

    # Resolved floor settings
    floor_enabled: bool = True
    default_floor_price: float = 0.0
    floor_currency: str = "USD"
    dynamic_floors_enabled: bool = False
    floor_adjustment_factor: float = 1.0
    banner_floor: float = 0.0
    video_floor: float = 0.0
    native_floor: float = 0.0
    audio_floor: float = 0.0

    # Resolved rate limit settings
    rate_limit_enabled: bool = True
    requests_per_second: int = 1000
    burst: int = 100

    # Resolved feature flags
    prebid_enabled: bool = True
    header_bidding_enabled: bool = True
    lazy_loading_enabled: bool = False
    refresh_enabled: bool = False
    refresh_interval_seconds: int = 30
    analytics_enabled: bool = True
    detailed_logging_enabled: bool = False
    ab_testing_enabled: bool = False
    ab_test_group: str = ""
    custom_features: dict[str, bool] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        return {
            "config_id": self.config_id,
            "config_level": self.config_level.value,
            "resolution_chain": self.resolution_chain,
            "idr": {
                "enabled": self.idr_enabled,
                "bypass_enabled": self.bypass_enabled,
                "shadow_mode": self.shadow_mode,
                "max_bidders": self.max_bidders,
                "min_score_threshold": self.min_score_threshold,
                "exploration_enabled": self.exploration_enabled,
                "exploration_rate": self.exploration_rate,
                "exploration_slots": self.exploration_slots,
                "low_confidence_threshold": self.low_confidence_threshold,
                "exploration_confidence_threshold": self.exploration_confidence_threshold,
                "anchor_bidders_enabled": self.anchor_bidders_enabled,
                "anchor_bidder_count": self.anchor_bidder_count,
                "custom_anchor_bidders": self.custom_anchor_bidders,
                "diversity_enabled": self.diversity_enabled,
                "diversity_categories": self.diversity_categories,
                "scoring_weights": self.scoring_weights,
                "latency_excellent_ms": self.latency_excellent_ms,
                "latency_poor_ms": self.latency_poor_ms,
                "selection_timeout_ms": self.selection_timeout_ms,
            },
            "privacy": {
                "enabled": self.privacy_enabled,
                "strict_mode": self.privacy_strict_mode,
                "gdpr_applies": self.gdpr_applies,
                "ccpa_applies": self.ccpa_applies,
                "coppa_applies": self.coppa_applies,
                "require_consent": self.require_consent,
                "allow_legitimate_interest": self.allow_legitimate_interest,
            },
            "bidders": {
                "enabled_bidders": self.enabled_bidders,
                "disabled_bidders": self.disabled_bidders,
                "bidder_params": self.bidder_params,
                "bidder_allowlist": self.bidder_allowlist,
                "bidder_blocklist": self.bidder_blocklist,
            },
            "floors": {
                "enabled": self.floor_enabled,
                "default_floor_price": self.default_floor_price,
                "floor_currency": self.floor_currency,
                "dynamic_floors_enabled": self.dynamic_floors_enabled,
                "floor_adjustment_factor": self.floor_adjustment_factor,
                "banner_floor": self.banner_floor,
                "video_floor": self.video_floor,
                "native_floor": self.native_floor,
                "audio_floor": self.audio_floor,
            },
            "rate_limits": {
                "enabled": self.rate_limit_enabled,
                "requests_per_second": self.requests_per_second,
                "burst": self.burst,
            },
            "features": {
                "prebid_enabled": self.prebid_enabled,
                "header_bidding_enabled": self.header_bidding_enabled,
                "lazy_loading_enabled": self.lazy_loading_enabled,
                "refresh_enabled": self.refresh_enabled,
                "refresh_interval_seconds": self.refresh_interval_seconds,
                "analytics_enabled": self.analytics_enabled,
                "detailed_logging_enabled": self.detailed_logging_enabled,
                "ab_testing_enabled": self.ab_testing_enabled,
                "ab_test_group": self.ab_test_group,
                "custom_features": self.custom_features,
            },
        }


def get_default_global_config() -> FeatureConfig:
    """
    Get the default global configuration.

    This provides sensible defaults for all settings.
    """
    return FeatureConfig(
        config_id="global",
        config_level=ConfigLevel.GLOBAL,
        name="Global Default Configuration",
        description="Default settings for all publishers, sites, and ad units",
        enabled=True,
        idr=IDRSettings(
            enabled=True,
            bypass_enabled=False,
            shadow_mode=False,
            max_bidders=15,
            min_score_threshold=25.0,
            exploration_enabled=True,
            exploration_rate=0.1,
            exploration_slots=2,
            low_confidence_threshold=0.5,
            exploration_confidence_threshold=0.3,
            anchor_bidders_enabled=True,
            anchor_bidder_count=3,
            custom_anchor_bidders=[],
            diversity_enabled=True,
            diversity_categories=['premium', 'mid_tier', 'video_specialist', 'native'],
            scoring_weights={
                'win_rate': 0.25,
                'bid_rate': 0.20,
                'cpm': 0.15,
                'floor_clearance': 0.15,
                'latency': 0.10,
                'recency': 0.10,
                'id_match': 0.05,
            },
            latency_excellent_ms=100,
            latency_poor_ms=500,
            selection_timeout_ms=50,
        ),
        privacy=PrivacySettings(
            privacy_enabled=True,
            privacy_strict_mode=False,
            gdpr_applies=True,
            ccpa_applies=True,
            coppa_applies=False,
            require_consent=True,
            allow_legitimate_interest=True,
        ),
        bidders=BidderSettings(
            enabled_bidders=[],
            disabled_bidders=[],
            bidder_params={},
            bidder_allowlist=[],
            bidder_blocklist=[],
        ),
        floors=FloorSettings(
            floor_enabled=True,
            default_floor_price=0.0,
            floor_currency="USD",
            dynamic_floors_enabled=False,
            floor_adjustment_factor=1.0,
            banner_floor=0.0,
            video_floor=0.0,
            native_floor=0.0,
            audio_floor=0.0,
        ),
        rate_limits=RateLimitSettings(
            rate_limit_enabled=True,
            requests_per_second=1000,
            burst=100,
        ),
        features=FeatureFlags(
            prebid_enabled=True,
            header_bidding_enabled=True,
            lazy_loading_enabled=False,
            refresh_enabled=False,
            refresh_interval_seconds=30,
            analytics_enabled=True,
            detailed_logging_enabled=False,
            ab_testing_enabled=False,
            ab_test_group="",
            custom_features={},
        ),
    )
