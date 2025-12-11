"""Classified Request model for IDR."""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from ..privacy.consent_models import ConsentSignals


class AdFormat(str, Enum):
    """Supported ad formats."""
    BANNER = 'banner'
    VIDEO = 'video'
    NATIVE = 'native'
    AUDIO = 'audio'


class DeviceType(str, Enum):
    """Device type classification."""
    DESKTOP = 'desktop'
    MOBILE = 'mobile'
    TABLET = 'tablet'
    CTV = 'ctv'


class AdPosition(str, Enum):
    """Ad position on page."""
    ATF = 'atf'  # Above the fold
    BTF = 'btf'  # Below the fold
    UNKNOWN = 'unknown'


@dataclass
class ClassifiedRequest:
    """
    Classified and normalized bid request for scoring.

    This represents the output of the Request Classifier,
    containing all relevant features extracted from an OpenRTB request.
    """

    # Impression attributes
    impression_id: str
    ad_format: AdFormat
    ad_sizes: list[str] = field(default_factory=list)  # e.g., ['300x250', '320x50']
    position: AdPosition = AdPosition.UNKNOWN

    # Environment
    device_type: DeviceType = DeviceType.DESKTOP
    os: str = ''  # e.g., 'ios', 'android', 'windows'
    browser: str = ''  # e.g., 'chrome', 'safari'
    connection_type: str = ''  # e.g., 'wifi', '4g'

    # Geo
    country: str = 'unknown'  # ISO 3166-1 alpha-2
    region: str = ''  # ISO 3166-2
    dma: Optional[str] = None  # US only (Designated Market Area)

    # Publisher
    publisher_id: str = ''
    site_id: str = ''
    domain: str = ''
    page_type: Optional[str] = None  # e.g., 'article', 'homepage'
    categories: list[str] = field(default_factory=list)  # IAB categories

    # User
    user_ids: dict[str, str] = field(default_factory=dict)  # e.g., {'uid2': '...', 'id5': '...'}
    has_consent: bool = False
    consent_string: Optional[str] = None

    # Privacy / Consent (full signals)
    consent_signals: Optional["ConsentSignals"] = None

    # Context
    timestamp: datetime = field(default_factory=datetime.utcnow)
    hour_of_day: int = 0  # 0-23
    day_of_week: int = 0  # 0-6 (Monday=0)

    # Floors
    floor_price: Optional[float] = None
    floor_currency: str = 'USD'

    def __post_init__(self):
        """Set derived fields after initialization."""
        if self.timestamp:
            self.hour_of_day = self.timestamp.hour
            self.day_of_week = self.timestamp.weekday()

    @property
    def primary_ad_size(self) -> Optional[str]:
        """Return the primary (first) ad size."""
        return self.ad_sizes[0] if self.ad_sizes else None

    @property
    def is_mobile(self) -> bool:
        """Check if request is from mobile device."""
        return self.device_type in (DeviceType.MOBILE, DeviceType.TABLET)

    @property
    def has_user_ids(self) -> bool:
        """Check if any user IDs are present."""
        return bool(self.user_ids)

    @property
    def gdpr_applies(self) -> bool:
        """Check if GDPR applies to this request."""
        if self.consent_signals:
            return self.consent_signals.gdpr_applies
        return False

    @property
    def ccpa_applies(self) -> bool:
        """Check if CCPA applies to this request."""
        if self.consent_signals:
            return self.consent_signals.ccpa_applies
        return False

    @property
    def coppa_applies(self) -> bool:
        """Check if COPPA applies to this request."""
        if self.consent_signals:
            return self.consent_signals.coppa_applies
        return False

    @property
    def can_personalize(self) -> bool:
        """Check if personalized ads are allowed."""
        if self.consent_signals:
            return self.consent_signals.can_personalize_ads()
        return self.has_consent

    def to_dict(self) -> dict:
        """Convert to dictionary representation."""
        return {
            'impression_id': self.impression_id,
            'ad_format': self.ad_format.value,
            'ad_sizes': self.ad_sizes,
            'position': self.position.value,
            'device_type': self.device_type.value,
            'os': self.os,
            'browser': self.browser,
            'connection_type': self.connection_type,
            'country': self.country,
            'region': self.region,
            'dma': self.dma,
            'publisher_id': self.publisher_id,
            'site_id': self.site_id,
            'domain': self.domain,
            'page_type': self.page_type,
            'categories': self.categories,
            'user_ids': self.user_ids,
            'has_consent': self.has_consent,
            'consent_string': self.consent_string,
            'timestamp': self.timestamp.isoformat() if self.timestamp else None,
            'hour_of_day': self.hour_of_day,
            'day_of_week': self.day_of_week,
            'floor_price': self.floor_price,
            'floor_currency': self.floor_currency,
            'privacy': {
                'gdpr_applies': self.gdpr_applies,
                'ccpa_applies': self.ccpa_applies,
                'coppa_applies': self.coppa_applies,
                'can_personalize': self.can_personalize,
            } if self.consent_signals else None,
        }
