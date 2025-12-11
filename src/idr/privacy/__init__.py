"""
IDR Privacy Module - GDPR, CCPA, and GPP compliance.

Handles consent signal parsing and bidder filtering based on privacy regulations.
"""

from src.idr.privacy.consent_models import (
    ConsentSignals,
    TCFConsent,
    USPrivacy,
    GPPConsent,
    PrivacyRegulation,
)
from src.idr.privacy.privacy_filter import (
    PrivacyFilter,
    BidderPrivacyRequirements,
    PrivacyFilterResult,
)

__all__ = [
    'ConsentSignals',
    'TCFConsent',
    'USPrivacy',
    'GPPConsent',
    'PrivacyRegulation',
    'PrivacyFilter',
    'BidderPrivacyRequirements',
    'PrivacyFilterResult',
]
