"""
Consent signal models for privacy compliance.

Supports:
- GDPR TCF v2 (Transparency and Consent Framework)
- CCPA/CPRA (California Consumer Privacy Act)
- GPP (Global Privacy Platform)
"""

import base64
from dataclasses import dataclass, field
from enum import Enum, auto
from typing import Optional


class PrivacyRegulation(Enum):
    """Privacy regulations supported by the IDR."""
    GDPR = auto()      # EU General Data Protection Regulation
    CCPA = auto()      # California Consumer Privacy Act
    CPRA = auto()      # California Privacy Rights Act (supersedes CCPA)
    LGPD = auto()      # Brazil Lei Geral de Proteção de Dados
    PIPL = auto()      # China Personal Information Protection Law
    VIRGINIA = auto()  # Virginia Consumer Data Protection Act
    COLORADO = auto()  # Colorado Privacy Act
    CONNECTICUT = auto()  # Connecticut Data Privacy Act
    UTAH = auto()      # Utah Consumer Privacy Act


# TCF v2 Purpose IDs
class TCFPurpose(Enum):
    """IAB TCF v2 Purpose definitions."""
    STORE_ACCESS = 1           # Store and/or access information on a device
    BASIC_ADS = 2              # Select basic ads
    PERSONALIZED_ADS = 3       # Create a personalised ads profile
    PERSONALIZED_CONTENT = 4   # Select personalised content
    AD_MEASUREMENT = 5         # Create a personalised content profile
    CONTENT_MEASUREMENT = 6    # Measure ad performance
    MARKET_RESEARCH = 7        # Measure content performance
    PRODUCT_DEVELOPMENT = 8    # Understand audiences through statistics
    SPECIAL_FEATURE_GEO = 9    # Develop and improve products
    SPECIAL_FEATURE_DEVICE = 10  # Use limited data to select content


@dataclass
class TCFConsent:
    """
    TCF v2 consent signal parsed from consent string.

    The TCF consent string is a base64-encoded binary format.
    This class extracts key signals without full decoding.
    """
    raw_string: str
    version: int = 2

    # Consent flags
    has_consent: bool = False
    vendor_consent: set[int] = field(default_factory=set)
    purpose_consent: set[int] = field(default_factory=set)
    legitimate_interest: set[int] = field(default_factory=set)

    # Special features
    special_feature_optins: set[int] = field(default_factory=set)

    # Publisher restrictions
    publisher_restrictions: dict[int, dict] = field(default_factory=dict)

    # Metadata
    cmp_id: int = 0
    cmp_version: int = 0
    consent_screen: int = 0
    consent_language: str = "EN"
    vendor_list_version: int = 0

    @classmethod
    def parse(cls, consent_string: str) -> "TCFConsent":
        """
        Parse a TCF v2 consent string.

        Note: This is a simplified parser. For production, consider using
        a full TCF decoder library like `tcflib` or `consent-string`.
        """
        if not consent_string:
            return cls(raw_string="", has_consent=False)

        try:
            # TCF v2 strings start with a version indicator
            # Full parsing would decode the base64 and read bit fields
            # For now, we do basic validation and assume consent if string exists

            # Try to decode to validate format
            padding = 4 - (len(consent_string) % 4)
            if padding != 4:
                consent_string_padded = consent_string + ('=' * padding)
            else:
                consent_string_padded = consent_string

            decoded = base64.urlsafe_b64decode(consent_string_padded)

            # Extract version from first 6 bits
            if len(decoded) > 0:
                version = (decoded[0] >> 2) & 0x3F
            else:
                version = 2

            # For a valid TCF string, we assume basic consent
            # Real implementation would parse all bit fields
            return cls(
                raw_string=consent_string,
                version=version,
                has_consent=True,
                # Default purposes for basic ads (1, 2, 7, 9, 10 are common)
                purpose_consent={1, 2, 7, 9, 10},
            )

        except Exception:
            # Invalid string - no consent
            return cls(raw_string=consent_string, has_consent=False)

    def has_purpose_consent(self, purpose: TCFPurpose) -> bool:
        """Check if consent exists for a specific purpose."""
        return purpose.value in self.purpose_consent

    def has_vendor_consent(self, vendor_id: int) -> bool:
        """Check if consent exists for a specific vendor."""
        # If we haven't parsed vendor consent, assume consent if has_consent is True
        if not self.vendor_consent and self.has_consent:
            return True
        return vendor_id in self.vendor_consent

    def can_process_for_ads(self) -> bool:
        """Check if processing for basic advertising is allowed."""
        return self.has_consent and self.has_purpose_consent(TCFPurpose.BASIC_ADS)

    def can_personalize_ads(self) -> bool:
        """Check if personalized advertising is allowed."""
        return (
            self.has_consent and
            self.has_purpose_consent(TCFPurpose.PERSONALIZED_ADS)
        )


@dataclass
class USPrivacy:
    """
    US Privacy String (CCPA) parser.

    Format: 4 characters
    - Position 1: Specification version (currently "1")
    - Position 2: Explicit Notice/Opportunity to Opt Out (Y/N/-)
    - Position 3: Opt-Out Sale (Y/N/-)
    - Position 4: LSPA Covered Transaction (Y/N/-)

    Examples:
    - "1YNN" = Notice given, not opted out, not LSPA
    - "1YYN" = Notice given, opted out of sale
    - "1---" = Not applicable
    """
    raw_string: str
    version: int = 1

    # Signal values
    explicit_notice: Optional[bool] = None  # Y=True, N=False, -=None
    opt_out_sale: Optional[bool] = None     # Y=True (opted out), N=False
    lspa_covered: Optional[bool] = None     # Y=True, N=False

    @classmethod
    def parse(cls, us_privacy_string: str) -> "USPrivacy":
        """Parse a US Privacy (CCPA) string."""
        if not us_privacy_string or len(us_privacy_string) != 4:
            return cls(raw_string=us_privacy_string or "")

        def parse_char(c: str) -> Optional[bool]:
            if c == 'Y':
                return True
            elif c == 'N':
                return False
            return None  # '-' or invalid

        try:
            version = int(us_privacy_string[0])
        except ValueError:
            version = 1

        return cls(
            raw_string=us_privacy_string,
            version=version,
            explicit_notice=parse_char(us_privacy_string[1]),
            opt_out_sale=parse_char(us_privacy_string[2]),
            lspa_covered=parse_char(us_privacy_string[3]),
        )

    def has_opted_out(self) -> bool:
        """Check if user has opted out of data sale."""
        return self.opt_out_sale is True

    def can_sell_data(self) -> bool:
        """Check if data sale is permitted."""
        # Can sell if not opted out (N) or not applicable (-)
        return self.opt_out_sale is not True

    def notice_given(self) -> bool:
        """Check if explicit notice was given."""
        return self.explicit_notice is True


@dataclass
class GPPConsent:
    """
    Global Privacy Platform (GPP) consent signal.

    GPP is the newer IAB standard that encompasses multiple jurisdictions
    including GDPR (TCF), US state laws, and others.

    The GPP string contains:
    - Header with section IDs indicating which regulations apply
    - Section-specific consent data
    """
    raw_string: str

    # Applicable sections
    section_ids: list[int] = field(default_factory=list)

    # Parsed sections (section_id -> section data)
    sections: dict[int, dict] = field(default_factory=dict)

    # Common section IDs
    # 2 = TCF EU v2
    # 6 = USP v1 (CCPA)
    # 7 = US National (MSPA)
    # 8-12 = US State sections

    @classmethod
    def parse(cls, gpp_string: str) -> "GPPConsent":
        """
        Parse a GPP consent string.

        Note: Simplified parser. Full GPP parsing requires the IAB GPP SDK.
        """
        if not gpp_string:
            return cls(raw_string="")

        try:
            # GPP strings are tilde-delimited
            # Format: HEADER~SECTION1~SECTION2~...
            parts = gpp_string.split('~')

            if len(parts) < 1:
                return cls(raw_string=gpp_string)

            # Parse header to get section IDs
            # Header format varies - this is simplified
            section_ids = []

            # Try to extract section IDs from header
            # Real implementation would decode the binary header

            return cls(
                raw_string=gpp_string,
                section_ids=section_ids,
            )

        except Exception:
            return cls(raw_string=gpp_string)

    def has_tcf_section(self) -> bool:
        """Check if GPP contains a TCF EU section."""
        return 2 in self.section_ids

    def has_us_section(self) -> bool:
        """Check if GPP contains any US privacy section."""
        us_sections = {6, 7, 8, 9, 10, 11, 12}
        return bool(set(self.section_ids) & us_sections)

    def get_tcf_consent(self) -> Optional[TCFConsent]:
        """Extract TCF consent from GPP if present."""
        if 2 in self.sections:
            tcf_data = self.sections[2]
            # Convert to TCFConsent
            return TCFConsent(
                raw_string=tcf_data.get('raw', ''),
                has_consent=tcf_data.get('has_consent', False),
            )
        return None


@dataclass
class ConsentSignals:
    """
    Unified container for all consent signals from a bid request.

    Extracts and normalizes consent from:
    - regs.ext.gdpr + user.ext.consent (TCF)
    - regs.ext.us_privacy (CCPA)
    - regs.gpp + regs.gpp_sid (GPP)
    """
    # Raw signals from request
    gdpr_applies: bool = False
    ccpa_applies: bool = False
    coppa_applies: bool = False

    # Parsed consent objects
    tcf: Optional[TCFConsent] = None
    us_privacy: Optional[USPrivacy] = None
    gpp: Optional[GPPConsent] = None

    # Geographic context
    country: str = ""
    region: str = ""

    @classmethod
    def from_openrtb(cls, request: dict) -> "ConsentSignals":
        """
        Extract consent signals from an OpenRTB bid request.

        Args:
            request: OpenRTB 2.x bid request dict

        Returns:
            ConsentSignals with parsed consent data
        """
        regs = request.get('regs', {})
        regs_ext = regs.get('ext', {})
        user = request.get('user', {})
        user_ext = user.get('ext', {})
        geo = request.get('device', {}).get('geo', {})

        # GDPR / TCF
        gdpr_applies = regs_ext.get('gdpr', 0) == 1
        tcf_consent_string = user_ext.get('consent', '')
        tcf = TCFConsent.parse(tcf_consent_string) if tcf_consent_string else None

        # CCPA / US Privacy
        us_privacy_string = regs_ext.get('us_privacy', '')
        us_privacy = USPrivacy.parse(us_privacy_string) if us_privacy_string else None
        ccpa_applies = bool(us_privacy_string)

        # GPP
        gpp_string = regs.get('gpp', '')
        gpp = GPPConsent.parse(gpp_string) if gpp_string else None

        # COPPA
        coppa_applies = regs.get('coppa', 0) == 1

        return cls(
            gdpr_applies=gdpr_applies,
            ccpa_applies=ccpa_applies,
            coppa_applies=coppa_applies,
            tcf=tcf,
            us_privacy=us_privacy,
            gpp=gpp,
            country=geo.get('country', ''),
            region=geo.get('region', ''),
        )

    def has_valid_consent(self) -> bool:
        """Check if valid consent exists where required."""
        if self.gdpr_applies:
            return self.tcf is not None and self.tcf.has_consent
        return True

    def can_process_for_basic_ads(self) -> bool:
        """
        Check if basic ad processing is allowed.

        Returns True if:
        - GDPR applies and TCF consent for basic ads exists, OR
        - GDPR doesn't apply, AND
        - User hasn't opted out under CCPA
        """
        # Check GDPR/TCF
        if self.gdpr_applies:
            if not self.tcf or not self.tcf.can_process_for_ads():
                return False

        # Check CCPA opt-out
        if self.us_privacy and self.us_privacy.has_opted_out():
            return False

        # Check COPPA (no behavioral targeting for children)
        if self.coppa_applies:
            return False

        return True

    def can_personalize_ads(self) -> bool:
        """
        Check if personalized advertising is allowed.

        More restrictive than basic ads - requires explicit consent.
        """
        if not self.can_process_for_basic_ads():
            return False

        # GDPR requires purpose 3/4 consent for personalization
        if self.gdpr_applies:
            if not self.tcf or not self.tcf.can_personalize_ads():
                return False

        return True

    def can_share_with_vendor(self, vendor_id: int) -> bool:
        """
        Check if data can be shared with a specific vendor.

        Args:
            vendor_id: IAB Global Vendor List ID
        """
        if self.gdpr_applies:
            if not self.tcf or not self.tcf.has_vendor_consent(vendor_id):
                return False

        return True

    def get_restrictions_summary(self) -> dict:
        """Get a summary of privacy restrictions."""
        return {
            'gdpr_applies': self.gdpr_applies,
            'ccpa_applies': self.ccpa_applies,
            'coppa_applies': self.coppa_applies,
            'has_tcf_consent': self.tcf.has_consent if self.tcf else False,
            'ccpa_opted_out': self.us_privacy.has_opted_out() if self.us_privacy else False,
            'can_basic_ads': self.can_process_for_basic_ads(),
            'can_personalize': self.can_personalize_ads(),
        }
