"""
OpenRTB Bidder Configuration Models

These dataclasses define the configuration structure for custom OpenRTB bidders.
Based on OpenRTB 2.6 specification from IAB Tech Lab.

Reference: https://github.com/InteractiveAdvertisingBureau/openrtb2.x
"""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any


class BidderStatus(str, Enum):
    """Bidder operational status."""

    ACTIVE = "active"  # Bidder is enabled and receiving traffic
    PAUSED = "paused"  # Temporarily disabled
    TESTING = "testing"  # Only receives test traffic
    DISABLED = "disabled"  # Fully disabled


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
    auth_type: str | None = None  # "none", "basic", "bearer", "header"
    auth_username: str | None = None
    auth_password: str | None = None
    auth_token: str | None = None
    auth_header_name: str | None = None
    auth_header_value: str | None = None

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
    video_max_duration: int | None = None
    video_min_duration: int | None = None

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
class SChainNode:
    """
    Supply Chain node for schain augmentation.

    Per IAB OpenRTB Supply Chain spec:
    https://iabtechlab.com/standards/openrtb-supply-chain/

    Attributes:
        asi: Canonical domain name of the SSP, Exchange, Header Wrapper, etc.
        sid: Identifier associated with the seller or reseller account.
        hp: Indicates if this node is involved in payment flow (1=yes, 0=no).
        rid: Optional request ID for tracking (typically omitted).
        name: Optional human-readable name of the entity.
        domain: Optional domain of the entity (may differ from asi).
        ext: Optional extension object for custom data.
    """
    asi: str
    sid: str
    hp: int = 1  # 1 = direct/payment flow, 0 = indirect
    rid: Optional[str] = None
    name: Optional[str] = None
    domain: Optional[str] = None
    ext: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        result = {
            "asi": self.asi,
            "sid": self.sid,
            "hp": self.hp,
        }
        if self.rid:
            result["rid"] = self.rid
        if self.name:
            result["name"] = self.name
        if self.domain:
            result["domain"] = self.domain
        if self.ext:
            result["ext"] = self.ext
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "SChainNode":
        """Create from dictionary."""
        return cls(
            asi=data.get("asi", ""),
            sid=data.get("sid", ""),
            hp=data.get("hp", 1),
            rid=data.get("rid"),
            name=data.get("name"),
            domain=data.get("domain"),
            ext=data.get("ext", {}),
        )


@dataclass
class SChainAugmentation:
    """
    Supply Chain augmentation configuration for per-bidder schain modification.

    This allows adding supply chain nodes to bid requests sent to specific bidders,
    enabling proper transparency and ads.txt/sellers.json compliance.

    Attributes:
        enabled: Whether schain augmentation is enabled for this bidder.
        nodes: List of supply chain nodes to append to the request's schain.
        complete: Override the schain complete flag (None = preserve original).
        version: SChain version string (default "1.0").
    """
    enabled: bool = False
    nodes: list[SChainNode] = field(default_factory=list)
    complete: Optional[int] = None  # None = preserve, 0 = incomplete, 1 = complete
    version: str = "1.0"

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for serialization."""
        return {
            "enabled": self.enabled,
            "nodes": [node.to_dict() for node in self.nodes],
            "complete": self.complete,
            "version": self.version,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "SChainAugmentation":
        """Create from dictionary."""
        nodes_data = data.get("nodes", [])
        nodes = [SChainNode.from_dict(n) for n in nodes_data]
        return cls(
            enabled=data.get("enabled", False),
            nodes=nodes,
            complete=data.get("complete"),
            version=data.get("version", "1.0"),
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
        schain_augment: Supply chain augmentation configuration
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
    seat_id: str | None = None

    # Supply chain augmentation
    schain_augment: SChainAugmentation = field(default_factory=SChainAugmentation)

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
            "schain_augment": self.schain_augment.to_dict(),
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
            schain_augment=SChainAugmentation.from_dict(data.get("schain_augment", {})),
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

    # Instance identification (for multi-instance support)
    bidder_family: str = ""  # Base bidder type (e.g., "appnexus" for appnexus_2)
    instance_number: int = 1  # Instance number (1, 2, 3...)
    publisher_id: str | None = None  # If set, publisher-specific instance

    # Optional fields
    description: str = ""
    capabilities: BidderCapabilities = field(default_factory=BidderCapabilities)
    rate_limits: BidderRateLimits = field(default_factory=BidderRateLimits)
    request_transform: RequestTransform = field(default_factory=RequestTransform)
    response_transform: ResponseTransform = field(default_factory=ResponseTransform)

    # Status and metadata
    status: BidderStatus = BidderStatus.TESTING
    gvl_vendor_id: int | None = None  # IAB GVL ID
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
    created_at: str | None = None
    updated_at: str | None = None
    last_active_at: str | None = None

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

        # Set bidder_family if not provided (extract from bidder_code)
        if not self.bidder_family:
            self.bidder_family = self._extract_family(self.bidder_code)
            self.instance_number = self._extract_instance_number(self.bidder_code)

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

    @staticmethod
    def _extract_family(bidder_code: str) -> str:
        """
        Extract bidder family from bidder code.

        Examples:
            "appnexus" -> "appnexus"
            "appnexus-2" -> "appnexus"
            "rubicon-3" -> "rubicon"
        """
        import re

        # Check if code ends with -N (instance suffix)
        match = re.match(r"^(.+)-(\d+)$", bidder_code)
        if match:
            return match.group(1)
        return bidder_code

    @staticmethod
    def _extract_instance_number(bidder_code: str) -> int:
        """
        Extract instance number from bidder code.

        Examples:
            "appnexus" -> 1
            "appnexus-2" -> 2
            "rubicon-3" -> 3
        """
        import re

        # Check if code ends with -N (instance suffix)
        match = re.match(r"^(.+)-(\d+)$", bidder_code)
        if match:
            return int(match.group(2))
        return 1

    @property
    def is_publisher_specific(self) -> bool:
        """Check if this is a publisher-specific bidder instance."""
        return self.publisher_id is not None

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
            "bidder_family": self.bidder_family,
            "instance_number": self.instance_number,
            "publisher_id": self.publisher_id,
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
            bidder_family=data.get("bidder_family", ""),
            instance_number=data.get("instance_number", 1),
            publisher_id=data.get("publisher_id"),
            endpoint=BidderEndpoint.from_dict(data.get("endpoint", {})),
            capabilities=BidderCapabilities.from_dict(data.get("capabilities", {})),
            rate_limits=BidderRateLimits.from_dict(data.get("rate_limits", {})),
            request_transform=RequestTransform.from_dict(
                data.get("request_transform", {})
            ),
            response_transform=ResponseTransform.from_dict(
                data.get("response_transform", {})
            ),
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
