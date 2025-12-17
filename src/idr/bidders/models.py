"""
OpenRTB Bidder Configuration Models

These dataclasses define the configuration structure for custom OpenRTB bidders.
Based on OpenRTB 2.6 specification from IAB Tech Lab.

Reference: https://github.com/InteractiveAdvertisingBureau/openrtb2.x
"""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Optional


class BidderStatus(str, Enum):
    """Bidder operational status."""
    ACTIVE = "active"           # Bidder is enabled and receiving traffic
    PAUSED = "paused"           # Temporarily disabled
    TESTING = "testing"         # Only receives test traffic
    DISABLED = "disabled"       # Fully disabled


class MediaType(str, Enum):
    """Supported media types per OpenRTB 2.6."""
    BANNER = "banner"
    VIDEO = "video"
    AUDIO = "audio"
    NATIVE = "native"


class AuctionType(int, Enum):
    """OpenRTB auction types."""
    FIRST_PRICE = 1
    SECOND_PRICE = 2


class ProtocolVersion(str, Enum):
    """Supported OpenRTB protocol versions."""
    ORTB_2_5 = "2.5"
    ORTB_2_6 = "2.6"


@dataclass
class BidderEndpoint:
    """
    Bidder endpoint configuration.

    Attributes:
        url: The bidder's OpenRTB endpoint URL
        method: HTTP method (POST for OpenRTB)
        timeout_ms: Request timeout in milliseconds
        protocol_version: OpenRTB version (2.5 or 2.6)
    """
    url: str
    method: str = "POST"
    timeout_ms: int = 200
    protocol_version: str = "2.6"

    # Optional authentication
    auth_type: Optional[str] = None  # "none", "basic", "bearer", "header"
    auth_username: Optional[str] = None
    auth_password: Optional[str] = None
    auth_token: Optional[str] = None
    auth_header_name: Optional[str] = None
    auth_header_value: Optional[str] = None

    # Additional headers
    custom_headers: dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "url": self.url,
            "method": self.method,
            "timeout_ms": self.timeout_ms,
            "protocol_version": self.protocol_version,
            "auth_type": self.auth_type,
            "auth_username": self.auth_username,
            "auth_password": self.auth_password,
            "auth_token": self.auth_token,
            "auth_header_name": self.auth_header_name,
            "auth_header_value": self.auth_header_value,
            "custom_headers": self.custom_headers,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "BidderEndpoint":
        """Create from dictionary."""
        return cls(
            url=data.get("url", ""),
            method=data.get("method", "POST"),
            timeout_ms=data.get("timeout_ms", 200),
            protocol_version=data.get("protocol_version", "2.6"),
            auth_type=data.get("auth_type"),
            auth_username=data.get("auth_username"),
            auth_password=data.get("auth_password"),
            auth_token=data.get("auth_token"),
            auth_header_name=data.get("auth_header_name"),
            auth_header_value=data.get("auth_header_value"),
            custom_headers=data.get("custom_headers", {}),
        )


@dataclass
class BidderCapabilities:
    """
    Bidder capabilities and supported features.

    Attributes:
        media_types: List of supported media types (banner, video, audio, native)
        currencies: Supported currencies (ISO 4217 codes)
        protocols: Supported video protocols (VAST versions)
        apis: Supported APIs (VPAID, MRAID, etc.)
    """
    media_types: list[str] = field(default_factory=lambda: ["banner"])
    currencies: list[str] = field(default_factory=lambda: ["USD"])

    # Site vs App support
    site_enabled: bool = True
    app_enabled: bool = True

    # Video capabilities (per OpenRTB 2.6)
    video_protocols: list[int] = field(default_factory=list)  # VAST versions
    video_mimes: list[str] = field(default_factory=list)
    video_linearity: list[int] = field(default_factory=list)
    video_playback_methods: list[int] = field(default_factory=list)
    video_max_duration: Optional[int] = None
    video_min_duration: Optional[int] = None

    # Banner capabilities
    banner_mimes: list[str] = field(default_factory=lambda: ["text/html"])
    banner_expdir: list[int] = field(default_factory=list)  # Expandable directions

    # Native capabilities
    native_api: list[int] = field(default_factory=list)

    # Audio capabilities
    audio_mimes: list[str] = field(default_factory=list)
    audio_protocols: list[int] = field(default_factory=list)

    # Advanced features
    supports_gdpr: bool = True
    supports_ccpa: bool = True
    supports_coppa: bool = True
    supports_gpp: bool = True
    supports_schain: bool = True
    supports_eids: bool = True
    supports_first_party_data: bool = True

    # CTV / Ad Pod support (OpenRTB 2.6)
    supports_ctv: bool = False
    supports_ad_pods: bool = False
    supports_dooh: bool = False

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "media_types": self.media_types,
            "currencies": self.currencies,
            "site_enabled": self.site_enabled,
            "app_enabled": self.app_enabled,
            "video_protocols": self.video_protocols,
            "video_mimes": self.video_mimes,
            "video_linearity": self.video_linearity,
            "video_playback_methods": self.video_playback_methods,
            "video_max_duration": self.video_max_duration,
            "video_min_duration": self.video_min_duration,
            "banner_mimes": self.banner_mimes,
            "banner_expdir": self.banner_expdir,
            "native_api": self.native_api,
            "audio_mimes": self.audio_mimes,
            "audio_protocols": self.audio_protocols,
            "supports_gdpr": self.supports_gdpr,
            "supports_ccpa": self.supports_ccpa,
            "supports_coppa": self.supports_coppa,
            "supports_gpp": self.supports_gpp,
            "supports_schain": self.supports_schain,
            "supports_eids": self.supports_eids,
            "supports_first_party_data": self.supports_first_party_data,
            "supports_ctv": self.supports_ctv,
            "supports_ad_pods": self.supports_ad_pods,
            "supports_dooh": self.supports_dooh,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "BidderCapabilities":
        """Create from dictionary."""
        return cls(
            media_types=data.get("media_types", ["banner"]),
            currencies=data.get("currencies", ["USD"]),
            site_enabled=data.get("site_enabled", True),
            app_enabled=data.get("app_enabled", True),
            video_protocols=data.get("video_protocols", []),
            video_mimes=data.get("video_mimes", []),
            video_linearity=data.get("video_linearity", []),
            video_playback_methods=data.get("video_playback_methods", []),
            video_max_duration=data.get("video_max_duration"),
            video_min_duration=data.get("video_min_duration"),
            banner_mimes=data.get("banner_mimes", ["text/html"]),
            banner_expdir=data.get("banner_expdir", []),
            native_api=data.get("native_api", []),
            audio_mimes=data.get("audio_mimes", []),
            audio_protocols=data.get("audio_protocols", []),
            supports_gdpr=data.get("supports_gdpr", True),
            supports_ccpa=data.get("supports_ccpa", True),
            supports_coppa=data.get("supports_coppa", True),
            supports_gpp=data.get("supports_gpp", True),
            supports_schain=data.get("supports_schain", True),
            supports_eids=data.get("supports_eids", True),
            supports_first_party_data=data.get("supports_first_party_data", True),
            supports_ctv=data.get("supports_ctv", False),
            supports_ad_pods=data.get("supports_ad_pods", False),
            supports_dooh=data.get("supports_dooh", False),
        )


@dataclass
class BidderRateLimits:
    """
    Rate limiting configuration for the bidder.

    Attributes:
        qps_limit: Maximum queries per second
        daily_limit: Maximum requests per day (0 = unlimited)
        concurrent_limit: Maximum concurrent requests
    """
    qps_limit: int = 1000
    daily_limit: int = 0  # 0 = unlimited
    concurrent_limit: int = 100

    # Current counters (not persisted)
    current_qps: int = 0
    daily_count: int = 0
    concurrent_count: int = 0

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "qps_limit": self.qps_limit,
            "daily_limit": self.daily_limit,
            "concurrent_limit": self.concurrent_limit,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "BidderRateLimits":
        """Create from dictionary."""
        return cls(
            qps_limit=data.get("qps_limit", 1000),
            daily_limit=data.get("daily_limit", 0),
            concurrent_limit=data.get("concurrent_limit", 100),
        )


@dataclass
class RequestTransform:
    """
    Request transformation rules to adapt the OpenRTB request
    for bidder-specific requirements.

    Attributes:
        field_mappings: Map standard fields to bidder-specific names
        field_additions: Add custom fields to the request
        field_removals: Fields to remove from the request
        imp_ext_template: Template for imp.ext object
        request_ext_template: Template for request.ext object
    """
    field_mappings: dict[str, str] = field(default_factory=dict)
    field_additions: dict[str, Any] = field(default_factory=dict)
    field_removals: list[str] = field(default_factory=list)

    # Extension templates (can include {{placeholders}})
    imp_ext_template: dict[str, Any] = field(default_factory=dict)
    request_ext_template: dict[str, Any] = field(default_factory=dict)
    site_ext_template: dict[str, Any] = field(default_factory=dict)
    user_ext_template: dict[str, Any] = field(default_factory=dict)

    # Bidder-specific seat ID
    seat_id: Optional[str] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "field_mappings": self.field_mappings,
            "field_additions": self.field_additions,
            "field_removals": self.field_removals,
            "imp_ext_template": self.imp_ext_template,
            "request_ext_template": self.request_ext_template,
            "site_ext_template": self.site_ext_template,
            "user_ext_template": self.user_ext_template,
            "seat_id": self.seat_id,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "RequestTransform":
        """Create from dictionary."""
        return cls(
            field_mappings=data.get("field_mappings", {}),
            field_additions=data.get("field_additions", {}),
            field_removals=data.get("field_removals", []),
            imp_ext_template=data.get("imp_ext_template", {}),
            request_ext_template=data.get("request_ext_template", {}),
            site_ext_template=data.get("site_ext_template", {}),
            user_ext_template=data.get("user_ext_template", {}),
            seat_id=data.get("seat_id"),
        )


@dataclass
class ResponseTransform:
    """
    Response transformation rules to normalize bidder responses
    to standard OpenRTB format.

    Attributes:
        bid_field_mappings: Map bidder-specific bid fields to standard
        price_adjustment: Price adjustment (e.g., 0.95 for 5% fee)
        currency_conversion: Whether to convert currencies
    """
    bid_field_mappings: dict[str, str] = field(default_factory=dict)
    price_adjustment: float = 1.0  # Multiplier for bid price
    currency_conversion: bool = True

    # Creative handling
    creative_type_mappings: dict[str, str] = field(default_factory=dict)
    extract_duration_from_vast: bool = True

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "bid_field_mappings": self.bid_field_mappings,
            "price_adjustment": self.price_adjustment,
            "currency_conversion": self.currency_conversion,
            "creative_type_mappings": self.creative_type_mappings,
            "extract_duration_from_vast": self.extract_duration_from_vast,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "ResponseTransform":
        """Create from dictionary."""
        return cls(
            bid_field_mappings=data.get("bid_field_mappings", {}),
            price_adjustment=data.get("price_adjustment", 1.0),
            currency_conversion=data.get("currency_conversion", True),
            creative_type_mappings=data.get("creative_type_mappings", {}),
            extract_duration_from_vast=data.get("extract_duration_from_vast", True),
        )


@dataclass
class BidderConfig:
    """
    Complete configuration for an OpenRTB bidder.

    This is the main configuration object stored in Redis and used
    by the Go adapter to make bid requests.

    Attributes:
        bidder_code: Unique identifier for this bidder (lowercase, no spaces)
        name: Human-readable bidder name
        description: Description of the bidder/DSP
        endpoint: Endpoint configuration
        capabilities: Supported features and media types
        rate_limits: Rate limiting configuration
        request_transform: Request transformation rules
        response_transform: Response transformation rules
        status: Current operational status
        gvl_vendor_id: IAB Global Vendor List ID (for privacy)
        priority: Bidder priority (higher = preferred)
    """
    bidder_code: str
    name: str
    endpoint: BidderEndpoint

    # Optional fields
    description: str = ""
    capabilities: BidderCapabilities = field(default_factory=BidderCapabilities)
    rate_limits: BidderRateLimits = field(default_factory=BidderRateLimits)
    request_transform: RequestTransform = field(default_factory=RequestTransform)
    response_transform: ResponseTransform = field(default_factory=ResponseTransform)

    # Status and metadata
    status: BidderStatus = BidderStatus.TESTING
    gvl_vendor_id: Optional[int] = None  # IAB GVL ID
    priority: int = 50  # 0-100, higher = more preferred

    # Contact information
    maintainer_email: str = ""
    maintainer_name: str = ""

    # Publisher restrictions
    allowed_publishers: list[str] = field(default_factory=list)  # Empty = all
    blocked_publishers: list[str] = field(default_factory=list)

    # Geo restrictions
    allowed_countries: list[str] = field(default_factory=list)  # Empty = all
    blocked_countries: list[str] = field(default_factory=list)

    # Timestamps
    created_at: Optional[str] = None
    updated_at: Optional[str] = None
    last_active_at: Optional[str] = None

    # Statistics (updated by system)
    total_requests: int = 0
    total_bids: int = 0
    total_wins: int = 0
    total_errors: int = 0
    avg_latency_ms: float = 0.0
    avg_bid_cpm: float = 0.0

    def __post_init__(self):
        """Validate and set defaults after initialization."""
        if not self.created_at:
            self.created_at = datetime.utcnow().isoformat()
        if not self.updated_at:
            self.updated_at = self.created_at

        # Normalize bidder_code
        self.bidder_code = self._normalize_code(self.bidder_code)

    @staticmethod
    def _normalize_code(code: str) -> str:
        """Normalize bidder code to lowercase alphanumeric with hyphens."""
        import re
        # Replace spaces and underscores with hyphens
        code = code.lower().replace(" ", "-").replace("_", "-")
        # Remove any non-alphanumeric characters except hyphens
        code = re.sub(r"[^a-z0-9-]", "", code)
        # Remove multiple consecutive hyphens
        code = re.sub(r"-+", "-", code)
        # Remove leading/trailing hyphens
        return code.strip("-")

    @property
    def is_enabled(self) -> bool:
        """Check if bidder is enabled for traffic."""
        return self.status in (BidderStatus.ACTIVE, BidderStatus.TESTING)

    @property
    def bid_rate(self) -> float:
        """Calculate bid rate."""
        if self.total_requests == 0:
            return 0.0
        return self.total_bids / self.total_requests

    @property
    def win_rate(self) -> float:
        """Calculate win rate."""
        if self.total_bids == 0:
            return 0.0
        return self.total_wins / self.total_bids

    @property
    def error_rate(self) -> float:
        """Calculate error rate."""
        if self.total_requests == 0:
            return 0.0
        return self.total_errors / self.total_requests

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "bidder_code": self.bidder_code,
            "name": self.name,
            "description": self.description,
            "endpoint": self.endpoint.to_dict(),
            "capabilities": self.capabilities.to_dict(),
            "rate_limits": self.rate_limits.to_dict(),
            "request_transform": self.request_transform.to_dict(),
            "response_transform": self.response_transform.to_dict(),
            "status": self.status.value,
            "gvl_vendor_id": self.gvl_vendor_id,
            "priority": self.priority,
            "maintainer_email": self.maintainer_email,
            "maintainer_name": self.maintainer_name,
            "allowed_publishers": self.allowed_publishers,
            "blocked_publishers": self.blocked_publishers,
            "allowed_countries": self.allowed_countries,
            "blocked_countries": self.blocked_countries,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
            "last_active_at": self.last_active_at,
            "total_requests": self.total_requests,
            "total_bids": self.total_bids,
            "total_wins": self.total_wins,
            "total_errors": self.total_errors,
            "avg_latency_ms": self.avg_latency_ms,
            "avg_bid_cpm": self.avg_bid_cpm,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "BidderConfig":
        """Create from dictionary."""
        return cls(
            bidder_code=data.get("bidder_code", ""),
            name=data.get("name", ""),
            description=data.get("description", ""),
            endpoint=BidderEndpoint.from_dict(data.get("endpoint", {})),
            capabilities=BidderCapabilities.from_dict(data.get("capabilities", {})),
            rate_limits=BidderRateLimits.from_dict(data.get("rate_limits", {})),
            request_transform=RequestTransform.from_dict(data.get("request_transform", {})),
            response_transform=ResponseTransform.from_dict(data.get("response_transform", {})),
            status=BidderStatus(data.get("status", "testing")),
            gvl_vendor_id=data.get("gvl_vendor_id"),
            priority=data.get("priority", 50),
            maintainer_email=data.get("maintainer_email", ""),
            maintainer_name=data.get("maintainer_name", ""),
            allowed_publishers=data.get("allowed_publishers", []),
            blocked_publishers=data.get("blocked_publishers", []),
            allowed_countries=data.get("allowed_countries", []),
            blocked_countries=data.get("blocked_countries", []),
            created_at=data.get("created_at"),
            updated_at=data.get("updated_at"),
            last_active_at=data.get("last_active_at"),
            total_requests=data.get("total_requests", 0),
            total_bids=data.get("total_bids", 0),
            total_wins=data.get("total_wins", 0),
            total_errors=data.get("total_errors", 0),
            avg_latency_ms=data.get("avg_latency_ms", 0.0),
            avg_bid_cpm=data.get("avg_bid_cpm", 0.0),
        )

    def to_json(self) -> str:
        """Convert to JSON string."""
        import json
        return json.dumps(self.to_dict())

    @classmethod
    def from_json(cls, json_str: str) -> "BidderConfig":
        """Create from JSON string."""
        import json
        return cls.from_dict(json.loads(json_str))
