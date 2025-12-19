"""
Privacy filter for bidder selection.

Filters bidders based on:
- GDPR consent requirements (TCF vendor consent)
- CCPA opt-out signals
- COPPA restrictions
- Regional privacy laws
"""

from dataclasses import dataclass, field
from enum import Enum, auto

from .consent_models import ConsentSignals, PrivacyRegulation


class FilterReason(Enum):
    """Reasons why a bidder was filtered for privacy."""

    ALLOWED = auto()
    NO_TCF_CONSENT = auto()
    NO_VENDOR_CONSENT = auto()
    NO_PURPOSE_CONSENT = auto()
    CCPA_OPT_OUT = auto()
    COPPA_RESTRICTED = auto()
    REGION_BLOCKED = auto()
    PERSONALIZATION_DENIED = auto()


@dataclass
class BidderPrivacyRequirements:
    """
    Privacy requirements for a specific bidder.

    Each bidder may have different requirements based on:
    - Their IAB Global Vendor List ID
    - Required TCF purposes
    - Regional restrictions
    - Processing type (basic vs personalized)
    """

    bidder_code: str

    # IAB Global Vendor List ID (for TCF)
    # See: https://iabeurope.eu/vendor-list/
    gvl_id: int | None = None

    # Required TCF purposes (must have consent for all)
    required_purposes: set[int] = field(default_factory=lambda: {1, 2})

    # Does this bidder require personalization consent?
    requires_personalization: bool = False

    # Regions where this bidder operates
    # Empty = all regions
    allowed_regions: set[str] = field(default_factory=set)
    blocked_regions: set[str] = field(default_factory=set)

    # Does this bidder support COPPA-compliant mode?
    supports_coppa: bool = False

    # Can operate without consent in non-GDPR regions?
    requires_consent_globally: bool = False


@dataclass
class PrivacyFilterResult:
    """Result of privacy filtering for a bidder."""

    bidder_code: str
    allowed: bool
    reason: FilterReason
    details: str = ""


# Known bidder privacy configurations
# GVL IDs from IAB Global Vendor List
BIDDER_PRIVACY_CONFIG: dict[str, BidderPrivacyRequirements] = {
    # Premium SSPs
    "appnexus": BidderPrivacyRequirements(
        bidder_code="appnexus",
        gvl_id=32,  # Xandr (AppNexus)
        required_purposes={1, 2, 7, 9, 10},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "rubicon": BidderPrivacyRequirements(
        bidder_code="rubicon",
        gvl_id=52,  # Magnite (Rubicon)
        required_purposes={1, 2, 7, 10},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "pubmatic": BidderPrivacyRequirements(
        bidder_code="pubmatic",
        gvl_id=76,
        required_purposes={1, 2, 7, 10},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "openx": BidderPrivacyRequirements(
        bidder_code="openx",
        gvl_id=69,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "ix": BidderPrivacyRequirements(
        bidder_code="ix",
        gvl_id=10,  # Index Exchange
        required_purposes={1, 2},
        requires_personalization=False,
        supports_coppa=True,
    ),
    # Mid-tier
    "triplelift": BidderPrivacyRequirements(
        bidder_code="triplelift",
        gvl_id=28,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "sovrn": BidderPrivacyRequirements(
        bidder_code="sovrn",
        gvl_id=13,
        required_purposes={1, 2},
        requires_personalization=False,
        supports_coppa=False,
    ),
    "sharethrough": BidderPrivacyRequirements(
        bidder_code="sharethrough",
        gvl_id=80,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "gumgum": BidderPrivacyRequirements(
        bidder_code="gumgum",
        gvl_id=61,
        required_purposes={1, 2},
        requires_personalization=False,
        supports_coppa=False,
    ),
    "33across": BidderPrivacyRequirements(
        bidder_code="33across",
        gvl_id=58,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=True,
    ),
    # Video specialists
    "spotx": BidderPrivacyRequirements(
        bidder_code="spotx",
        gvl_id=165,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "beachfront": BidderPrivacyRequirements(
        bidder_code="beachfront",
        gvl_id=335,
        required_purposes={1, 2},
        requires_personalization=False,
        supports_coppa=False,
    ),
    "unruly": BidderPrivacyRequirements(
        bidder_code="unruly",
        gvl_id=36,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=False,
    ),
    # Native specialists
    "teads": BidderPrivacyRequirements(
        bidder_code="teads",
        gvl_id=132,
        required_purposes={1, 2, 7, 10},
        requires_personalization=False,
        supports_coppa=True,
    ),
    "outbrain": BidderPrivacyRequirements(
        bidder_code="outbrain",
        gvl_id=164,
        required_purposes={1, 2},
        requires_personalization=False,
        supports_coppa=False,
    ),
    # Personalization-focused (require purpose 3/4)
    "criteo": BidderPrivacyRequirements(
        bidder_code="criteo",
        gvl_id=91,
        required_purposes={1, 2, 3, 4},
        requires_personalization=True,
        supports_coppa=False,
    ),
    "taboola": BidderPrivacyRequirements(
        bidder_code="taboola",
        gvl_id=42,
        required_purposes={1, 2, 3},
        requires_personalization=True,
        supports_coppa=False,
    ),
    # Additional SSPs
    "medianet": BidderPrivacyRequirements(
        bidder_code="medianet",
        gvl_id=142,
        required_purposes={1, 2},
        requires_personalization=False,
        supports_coppa=False,
    ),
    "conversant": BidderPrivacyRequirements(
        bidder_code="conversant",
        gvl_id=24,  # Epsilon
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=True,
    ),
    # Regional (EMEA focused)
    "adform": BidderPrivacyRequirements(
        bidder_code="adform",
        gvl_id=50,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=False,
        allowed_regions={"EU", "GB", "EEA"},
    ),
    "smartadserver": BidderPrivacyRequirements(
        bidder_code="smartadserver",
        gvl_id=45,
        required_purposes={1, 2},
        requires_personalization=False,
        supports_coppa=False,
    ),
    "improvedigital": BidderPrivacyRequirements(
        bidder_code="improvedigital",
        gvl_id=253,
        required_purposes={1, 2, 7},
        requires_personalization=False,
        supports_coppa=False,
    ),
}

# Default requirements for unknown bidders
DEFAULT_PRIVACY_REQUIREMENTS = BidderPrivacyRequirements(
    bidder_code="_default",
    gvl_id=None,
    required_purposes={1, 2},  # Basic storage and ads
    requires_personalization=False,
    supports_coppa=False,
)


class PrivacyFilter:
    """
    Filters bidders based on privacy consent signals.

    Checks each bidder against:
    - GDPR/TCF consent requirements
    - CCPA opt-out status
    - COPPA restrictions
    - Regional restrictions
    """

    def __init__(
        self,
        bidder_configs: dict[str, BidderPrivacyRequirements] | None = None,
        strict_mode: bool = False,
    ):
        """
        Initialize the privacy filter.

        Args:
            bidder_configs: Custom bidder privacy configs (uses defaults if None)
            strict_mode: If True, filter bidders with unknown GVL IDs
        """
        self.bidder_configs = bidder_configs or BIDDER_PRIVACY_CONFIG
        self.strict_mode = strict_mode

    def get_bidder_config(self, bidder_code: str) -> BidderPrivacyRequirements:
        """Get privacy requirements for a bidder."""
        return self.bidder_configs.get(
            bidder_code.lower(), DEFAULT_PRIVACY_REQUIREMENTS
        )

    def filter_bidder(
        self,
        bidder_code: str,
        consent: ConsentSignals,
    ) -> PrivacyFilterResult:
        """
        Check if a bidder is allowed based on consent signals.

        Args:
            bidder_code: The bidder to check
            consent: Consent signals from the request

        Returns:
            PrivacyFilterResult indicating if bidder is allowed
        """
        config = self.get_bidder_config(bidder_code)

        # Check COPPA first (most restrictive)
        if consent.coppa_applies:
            if not config.supports_coppa:
                return PrivacyFilterResult(
                    bidder_code=bidder_code,
                    allowed=False,
                    reason=FilterReason.COPPA_RESTRICTED,
                    details=f"{bidder_code} does not support COPPA-compliant mode",
                )

        # Check regional restrictions
        if config.allowed_regions:
            if consent.country not in config.allowed_regions:
                return PrivacyFilterResult(
                    bidder_code=bidder_code,
                    allowed=False,
                    reason=FilterReason.REGION_BLOCKED,
                    details=f"{bidder_code} not available in {consent.country}",
                )

        if config.blocked_regions:
            if consent.country in config.blocked_regions:
                return PrivacyFilterResult(
                    bidder_code=bidder_code,
                    allowed=False,
                    reason=FilterReason.REGION_BLOCKED,
                    details=f"{bidder_code} blocked in {consent.country}",
                )

        # Check GDPR/TCF
        if consent.gdpr_applies:
            result = self._check_tcf_consent(bidder_code, config, consent)
            if not result.allowed:
                return result

        # Check CCPA
        if consent.us_privacy:
            result = self._check_ccpa_consent(bidder_code, config, consent)
            if not result.allowed:
                return result

        # Check personalization requirements
        if config.requires_personalization:
            if not consent.can_personalize_ads():
                return PrivacyFilterResult(
                    bidder_code=bidder_code,
                    allowed=False,
                    reason=FilterReason.PERSONALIZATION_DENIED,
                    details=f"{bidder_code} requires personalization consent",
                )

        return PrivacyFilterResult(
            bidder_code=bidder_code,
            allowed=True,
            reason=FilterReason.ALLOWED,
        )

    def _check_tcf_consent(
        self,
        bidder_code: str,
        config: BidderPrivacyRequirements,
        consent: ConsentSignals,
    ) -> PrivacyFilterResult:
        """Check TCF/GDPR consent for a bidder."""
        # Must have TCF consent when GDPR applies
        if not consent.tcf or not consent.tcf.has_consent:
            return PrivacyFilterResult(
                bidder_code=bidder_code,
                allowed=False,
                reason=FilterReason.NO_TCF_CONSENT,
                details="GDPR applies but no valid TCF consent",
            )

        # Check vendor consent if GVL ID is known
        if config.gvl_id:
            if not consent.tcf.has_vendor_consent(config.gvl_id):
                return PrivacyFilterResult(
                    bidder_code=bidder_code,
                    allowed=False,
                    reason=FilterReason.NO_VENDOR_CONSENT,
                    details=f"No vendor consent for GVL ID {config.gvl_id}",
                )
        elif self.strict_mode:
            # In strict mode, reject bidders without known GVL ID
            return PrivacyFilterResult(
                bidder_code=bidder_code,
                allowed=False,
                reason=FilterReason.NO_VENDOR_CONSENT,
                details=f"{bidder_code} has no known GVL ID (strict mode)",
            )

        # Check required purposes
        for purpose_id in config.required_purposes:
            if purpose_id not in consent.tcf.purpose_consent:
                return PrivacyFilterResult(
                    bidder_code=bidder_code,
                    allowed=False,
                    reason=FilterReason.NO_PURPOSE_CONSENT,
                    details=f"Missing consent for TCF purpose {purpose_id}",
                )

        return PrivacyFilterResult(
            bidder_code=bidder_code,
            allowed=True,
            reason=FilterReason.ALLOWED,
        )

    def _check_ccpa_consent(
        self,
        bidder_code: str,
        config: BidderPrivacyRequirements,
        consent: ConsentSignals,
    ) -> PrivacyFilterResult:
        """Check CCPA consent for a bidder."""
        if consent.us_privacy and consent.us_privacy.has_opted_out():
            # User opted out of data sale
            # Some bidders may still work in non-sale mode, but we filter conservatively
            return PrivacyFilterResult(
                bidder_code=bidder_code,
                allowed=False,
                reason=FilterReason.CCPA_OPT_OUT,
                details="User opted out under CCPA",
            )

        return PrivacyFilterResult(
            bidder_code=bidder_code,
            allowed=True,
            reason=FilterReason.ALLOWED,
        )

    def filter_bidders(
        self,
        bidder_codes: list[str],
        consent: ConsentSignals,
    ) -> tuple[list[str], list[PrivacyFilterResult]]:
        """
        Filter a list of bidders based on consent.

        Args:
            bidder_codes: List of bidder codes to check
            consent: Consent signals from the request

        Returns:
            Tuple of (allowed_bidders, filter_results)
        """
        allowed = []
        results = []

        for bidder_code in bidder_codes:
            result = self.filter_bidder(bidder_code, consent)
            results.append(result)
            if result.allowed:
                allowed.append(bidder_code)

        return allowed, results

    def get_privacy_summary(
        self,
        consent: ConsentSignals,
        filter_results: list[PrivacyFilterResult],
    ) -> dict:
        """
        Get a summary of privacy filtering for logging/analytics.

        Args:
            consent: The consent signals used
            filter_results: Results from filtering

        Returns:
            Summary dict for logging
        """
        filtered_count = sum(1 for r in filter_results if not r.allowed)
        reasons = {}
        for r in filter_results:
            if not r.allowed:
                reason_name = r.reason.name
                reasons[reason_name] = reasons.get(reason_name, 0) + 1

        return {
            "total_bidders": len(filter_results),
            "allowed": len(filter_results) - filtered_count,
            "filtered": filtered_count,
            "filter_reasons": reasons,
            "gdpr_applies": consent.gdpr_applies,
            "ccpa_applies": consent.ccpa_applies,
            "coppa_applies": consent.coppa_applies,
            "has_tcf_consent": consent.tcf.has_consent if consent.tcf else False,
            "ccpa_opted_out": consent.us_privacy.has_opted_out()
            if consent.us_privacy
            else False,
        }


# EU/EEA country codes for GDPR applicability
EU_EEA_COUNTRIES = {
    "AT",
    "BE",
    "BG",
    "HR",
    "CY",
    "CZ",
    "DK",
    "EE",
    "FI",
    "FR",
    "DE",
    "GR",
    "HU",
    "IE",
    "IT",
    "LV",
    "LT",
    "LU",
    "MT",
    "NL",
    "PL",
    "PT",
    "RO",
    "SK",
    "SI",
    "ES",
    "SE",  # EU members
    "IS",
    "LI",
    "NO",  # EEA
    "GB",  # UK (still applies GDPR)
}

# US states with comprehensive privacy laws
US_PRIVACY_STATES = {
    "CA",  # California (CCPA/CPRA)
    "VA",  # Virginia (VCDPA)
    "CO",  # Colorado (CPA)
    "CT",  # Connecticut (CTDPA)
    "UT",  # Utah (UCPA)
}


def infer_privacy_jurisdiction(
    country: str, region: str = ""
) -> set[PrivacyRegulation]:
    """
    Infer applicable privacy regulations based on geography.

    Args:
        country: ISO 3166-1 alpha-2 country code
        region: ISO 3166-2 region code (e.g., "CA" for California)

    Returns:
        Set of applicable privacy regulations
    """
    regulations = set()

    if country in EU_EEA_COUNTRIES:
        regulations.add(PrivacyRegulation.GDPR)

    if country == "BR":
        regulations.add(PrivacyRegulation.LGPD)

    if country == "CN":
        regulations.add(PrivacyRegulation.PIPL)

    if country == "US":
        if region == "CA":
            regulations.add(PrivacyRegulation.CCPA)
        elif region == "VA":
            regulations.add(PrivacyRegulation.VIRGINIA)
        elif region == "CO":
            regulations.add(PrivacyRegulation.COLORADO)
        elif region == "CT":
            regulations.add(PrivacyRegulation.CONNECTICUT)
        elif region == "UT":
            regulations.add(PrivacyRegulation.UTAH)

    return regulations
