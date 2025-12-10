"""Tests for privacy compliance (GDPR, CCPA, COPPA)."""

import pytest

from src.idr.privacy.consent_models import (
    ConsentSignals,
    GPPConsent,
    TCFConsent,
    TCFPurpose,
    USPrivacy,
)
from src.idr.privacy.privacy_filter import (
    BIDDER_PRIVACY_CONFIG,
    BidderPrivacyRequirements,
    FilterReason,
    PrivacyFilter,
    infer_privacy_jurisdiction,
)


class TestTCFConsent:
    """Tests for TCF consent string parsing."""

    def test_empty_string_no_consent(self):
        """Empty string should result in no consent."""
        tcf = TCFConsent.parse("")
        assert tcf.has_consent is False

    def test_valid_string_has_consent(self):
        """Valid TCF string should have consent."""
        # This is a sample TCF v2 consent string
        tcf_string = "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"
        tcf = TCFConsent.parse(tcf_string)
        assert tcf.has_consent is True
        assert tcf.version == 2

    def test_can_process_for_ads_with_consent(self):
        """Should allow basic ads with consent."""
        tcf = TCFConsent(
            raw_string="test",
            has_consent=True,
            purpose_consent={1, 2, 7},
        )
        assert tcf.can_process_for_ads() is True

    def test_can_process_for_ads_without_consent(self):
        """Should not allow basic ads without consent."""
        tcf = TCFConsent(
            raw_string="test",
            has_consent=False,
        )
        assert tcf.can_process_for_ads() is False

    def test_can_personalize_ads(self):
        """Should check purpose 3/4 for personalization."""
        tcf_no_personal = TCFConsent(
            raw_string="test",
            has_consent=True,
            purpose_consent={1, 2},
        )
        assert tcf_no_personal.can_personalize_ads() is False

        tcf_with_personal = TCFConsent(
            raw_string="test",
            has_consent=True,
            purpose_consent={1, 2, 3, 4},
        )
        assert tcf_with_personal.can_personalize_ads() is True

    def test_has_vendor_consent(self):
        """Should check vendor consent."""
        tcf = TCFConsent(
            raw_string="test",
            has_consent=True,
            vendor_consent={32, 52, 76},  # AppNexus, Rubicon, PubMatic
        )
        assert tcf.has_vendor_consent(32) is True
        assert tcf.has_vendor_consent(999) is False

    def test_vendor_consent_defaults_when_has_consent(self):
        """Should default to True for unknown vendors when has_consent is True."""
        tcf = TCFConsent(
            raw_string="test",
            has_consent=True,
            vendor_consent=set(),  # Empty - no specific vendors
        )
        # When has_consent is True but no specific vendors, assume consent
        assert tcf.has_vendor_consent(999) is True


class TestUSPrivacy:
    """Tests for CCPA/US Privacy string parsing."""

    def test_opted_out(self):
        """1YYN = notice given, opted out."""
        usp = USPrivacy.parse("1YYN")
        assert usp.explicit_notice is True
        assert usp.opt_out_sale is True
        assert usp.has_opted_out() is True
        assert usp.can_sell_data() is False

    def test_not_opted_out(self):
        """1YNN = notice given, not opted out."""
        usp = USPrivacy.parse("1YNN")
        assert usp.explicit_notice is True
        assert usp.opt_out_sale is False
        assert usp.has_opted_out() is False
        assert usp.can_sell_data() is True

    def test_not_applicable(self):
        """1--- = not applicable."""
        usp = USPrivacy.parse("1---")
        assert usp.explicit_notice is None
        assert usp.opt_out_sale is None
        assert usp.has_opted_out() is False
        assert usp.can_sell_data() is True

    def test_invalid_string(self):
        """Invalid strings should be handled gracefully."""
        usp = USPrivacy.parse("invalid")
        assert usp.raw_string == "invalid"


class TestConsentSignals:
    """Tests for unified consent signals extraction."""

    def test_from_openrtb_gdpr(self):
        """Should extract GDPR signals from OpenRTB."""
        request = {
            "regs": {
                "ext": {"gdpr": 1}
            },
            "user": {
                "ext": {
                    "consent": "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"
                }
            },
            "device": {
                "geo": {"country": "DE"}
            }
        }
        signals = ConsentSignals.from_openrtb(request)
        assert signals.gdpr_applies is True
        assert signals.tcf is not None
        assert signals.tcf.has_consent is True
        assert signals.country == "DE"

    def test_from_openrtb_ccpa(self):
        """Should extract CCPA signals from OpenRTB."""
        request = {
            "regs": {
                "ext": {"us_privacy": "1YYN"}
            },
            "device": {
                "geo": {"country": "US", "region": "CA"}
            }
        }
        signals = ConsentSignals.from_openrtb(request)
        assert signals.ccpa_applies is True
        assert signals.us_privacy is not None
        assert signals.us_privacy.has_opted_out() is True

    def test_from_openrtb_coppa(self):
        """Should extract COPPA signals from OpenRTB."""
        request = {
            "regs": {"coppa": 1}
        }
        signals = ConsentSignals.from_openrtb(request)
        assert signals.coppa_applies is True

    def test_can_process_for_basic_ads_gdpr_consent(self):
        """Should allow basic ads with GDPR consent."""
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(
                raw_string="test",
                has_consent=True,
                purpose_consent={1, 2, 7},
            ),
        )
        assert signals.can_process_for_basic_ads() is True

    def test_can_process_for_basic_ads_gdpr_no_consent(self):
        """Should block basic ads without GDPR consent."""
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(raw_string="", has_consent=False),
        )
        assert signals.can_process_for_basic_ads() is False

    def test_can_process_for_basic_ads_ccpa_opted_out(self):
        """Should block if CCPA opted out."""
        signals = ConsentSignals(
            ccpa_applies=True,
            us_privacy=USPrivacy(
                raw_string="1YYN",
                opt_out_sale=True,
            ),
        )
        assert signals.can_process_for_basic_ads() is False

    def test_can_process_for_basic_ads_coppa(self):
        """Should block basic ads for COPPA traffic."""
        signals = ConsentSignals(coppa_applies=True)
        assert signals.can_process_for_basic_ads() is False


class TestPrivacyFilter:
    """Tests for privacy-based bidder filtering."""

    @pytest.fixture
    def filter(self):
        """Create a privacy filter."""
        return PrivacyFilter()

    def test_filter_allows_with_consent(self, filter):
        """Should allow bidders when consent is given."""
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(
                raw_string="test",
                has_consent=True,
                purpose_consent={1, 2, 7, 9, 10},
            ),
        )
        result = filter.filter_bidder("appnexus", signals)
        assert result.allowed is True
        assert result.reason == FilterReason.ALLOWED

    def test_filter_blocks_without_tcf_consent(self, filter):
        """Should block bidders when GDPR applies but no TCF consent."""
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(raw_string="", has_consent=False),
        )
        result = filter.filter_bidder("appnexus", signals)
        assert result.allowed is False
        assert result.reason == FilterReason.NO_TCF_CONSENT

    def test_filter_blocks_ccpa_opt_out(self, filter):
        """Should block bidders when CCPA opted out."""
        signals = ConsentSignals(
            ccpa_applies=True,
            us_privacy=USPrivacy(
                raw_string="1YYN",
                opt_out_sale=True,
            ),
        )
        result = filter.filter_bidder("appnexus", signals)
        assert result.allowed is False
        assert result.reason == FilterReason.CCPA_OPT_OUT

    def test_filter_blocks_coppa_unsupported(self, filter):
        """Should block bidders that don't support COPPA."""
        signals = ConsentSignals(coppa_applies=True)
        # sovrn doesn't support COPPA
        result = filter.filter_bidder("sovrn", signals)
        assert result.allowed is False
        assert result.reason == FilterReason.COPPA_RESTRICTED

    def test_filter_allows_coppa_supported(self, filter):
        """Should allow bidders that support COPPA."""
        signals = ConsentSignals(coppa_applies=True)
        # appnexus supports COPPA
        result = filter.filter_bidder("appnexus", signals)
        assert result.allowed is True

    def test_filter_blocks_personalization_required(self, filter):
        """Should block personalization-focused bidders without consent."""
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(
                raw_string="test",
                has_consent=True,
                purpose_consent={1, 2},  # No purpose 3/4
            ),
        )
        # criteo requires personalization (purpose 3/4)
        # It will fail at purpose consent check first
        result = filter.filter_bidder("criteo", signals)
        assert result.allowed is False
        # Will be NO_PURPOSE_CONSENT since we check TCF purposes before personalization
        assert result.reason in (FilterReason.NO_PURPOSE_CONSENT, FilterReason.PERSONALIZATION_DENIED)

    def test_filter_multiple_bidders(self, filter):
        """Should filter multiple bidders at once."""
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(
                raw_string="test",
                has_consent=True,
                # Provide all standard purposes that most bidders need
                purpose_consent={1, 2, 7, 9, 10},
            ),
        )
        bidders = ["appnexus", "rubicon", "criteo", "pubmatic"]
        allowed, results = filter.filter_bidders(bidders, signals)

        # appnexus, rubicon, pubmatic should be allowed (have all required purposes)
        # criteo should be filtered (requires personalization - purpose 3/4)
        assert "appnexus" in allowed
        assert "rubicon" in allowed
        assert "pubmatic" in allowed
        assert "criteo" not in allowed

    def test_filter_returns_privacy_summary(self, filter):
        """Should return a privacy summary."""
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(
                raw_string="test",
                has_consent=True,
                purpose_consent={1, 2},
            ),
        )
        bidders = ["appnexus", "criteo"]
        _, results = filter.filter_bidders(bidders, signals)

        summary = filter.get_privacy_summary(signals, results)
        assert summary['gdpr_applies'] is True
        assert summary['total_bidders'] == 2
        assert summary['filtered'] >= 1

    def test_strict_mode_blocks_unknown_gvl(self):
        """Strict mode should block bidders with unknown GVL ID."""
        strict_filter = PrivacyFilter(strict_mode=True)
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(
                raw_string="test",
                has_consent=True,
                purpose_consent={1, 2, 7},
            ),
        )
        # unknown_bidder has no GVL ID
        result = strict_filter.filter_bidder("unknown_bidder", signals)
        assert result.allowed is False
        assert result.reason == FilterReason.NO_VENDOR_CONSENT


class TestBidderPrivacyConfig:
    """Tests for bidder privacy configuration."""

    def test_known_bidders_have_config(self):
        """Known bidders should have privacy config."""
        known_bidders = [
            "appnexus", "rubicon", "pubmatic", "openx", "ix",
            "triplelift", "criteo", "teads", "spotx"
        ]
        for bidder in known_bidders:
            assert bidder in BIDDER_PRIVACY_CONFIG

    def test_appnexus_config(self):
        """AppNexus should have correct GVL ID."""
        config = BIDDER_PRIVACY_CONFIG["appnexus"]
        assert config.gvl_id == 32
        assert config.supports_coppa is True

    def test_criteo_requires_personalization(self):
        """Criteo should require personalization consent."""
        config = BIDDER_PRIVACY_CONFIG["criteo"]
        assert config.requires_personalization is True
        assert 3 in config.required_purposes
        assert 4 in config.required_purposes


class TestInferPrivacyJurisdiction:
    """Tests for geographic privacy jurisdiction inference."""

    def test_eu_countries_have_gdpr(self):
        """EU countries should have GDPR."""
        from src.idr.privacy.privacy_filter import PrivacyRegulation, EU_EEA_COUNTRIES

        for country in ["DE", "FR", "IT", "ES", "NL"]:
            assert country in EU_EEA_COUNTRIES
            regs = infer_privacy_jurisdiction(country)
            assert PrivacyRegulation.GDPR in regs

    def test_california_has_ccpa(self):
        """California should have CCPA."""
        from src.idr.privacy.privacy_filter import PrivacyRegulation

        regs = infer_privacy_jurisdiction("US", "CA")
        assert PrivacyRegulation.CCPA in regs

    def test_brazil_has_lgpd(self):
        """Brazil should have LGPD."""
        from src.idr.privacy.privacy_filter import PrivacyRegulation

        regs = infer_privacy_jurisdiction("BR")
        assert PrivacyRegulation.LGPD in regs

    def test_china_has_pipl(self):
        """China should have PIPL."""
        from src.idr.privacy.privacy_filter import PrivacyRegulation

        regs = infer_privacy_jurisdiction("CN")
        assert PrivacyRegulation.PIPL in regs


class TestIntegrationWithClassifier:
    """Tests for privacy integration with request classifier."""

    def test_classifier_extracts_consent_signals(self):
        """Classifier should extract consent signals from OpenRTB."""
        from src.idr.classifier.request_classifier import RequestClassifier

        classifier = RequestClassifier()
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "regs": {
                "ext": {"gdpr": 1, "us_privacy": "1YNN"}
            },
            "user": {
                "ext": {"consent": "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"}
            },
            "device": {
                "geo": {"country": "DE"}
            }
        }

        classified = classifier.classify(request)

        assert classified.consent_signals is not None
        assert classified.gdpr_applies is True
        assert classified.consent_signals.tcf is not None
        assert classified.consent_signals.tcf.has_consent is True

    def test_classifier_handles_missing_consent(self):
        """Classifier should handle missing consent gracefully."""
        from src.idr.classifier.request_classifier import RequestClassifier

        classifier = RequestClassifier()
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
        }

        classified = classifier.classify(request)

        assert classified.consent_signals is not None
        assert classified.gdpr_applies is False
        assert classified.ccpa_applies is False


class TestIntegrationWithSelector:
    """Tests for privacy integration with partner selector."""

    def test_selector_applies_privacy_filter(self):
        """Selector should apply privacy filter when enabled."""
        from src.idr.selector.partner_selector import PartnerSelector, SelectorConfig
        from src.idr.models.bidder_score import BidderScore
        from src.idr.models.classified_request import ClassifiedRequest, AdFormat, DeviceType

        privacy_filter = PrivacyFilter()
        config = SelectorConfig(privacy_enabled=True, max_bidders=10)
        selector = PartnerSelector(config=config, privacy_filter=privacy_filter)

        # Create classified request with GDPR but no consent
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(raw_string="", has_consent=False),
        )
        request = ClassifiedRequest(
            impression_id="1",
            ad_format=AdFormat.BANNER,
            consent_signals=signals,
        )

        # Create scores
        scores = [
            BidderScore(bidder_code="appnexus", total_score=80, confidence=0.9),
            BidderScore(bidder_code="rubicon", total_score=75, confidence=0.9),
        ]

        result = selector.select_partners(scores, request)

        # All bidders should be filtered due to no GDPR consent
        assert result.privacy_filtered_count > 0
        assert len(result.selected) == 0

    def test_selector_allows_with_consent(self):
        """Selector should allow bidders with proper consent."""
        from src.idr.selector.partner_selector import PartnerSelector, SelectorConfig
        from src.idr.models.bidder_score import BidderScore
        from src.idr.models.classified_request import ClassifiedRequest, AdFormat, DeviceType

        privacy_filter = PrivacyFilter()
        config = SelectorConfig(privacy_enabled=True, max_bidders=10)
        selector = PartnerSelector(config=config, privacy_filter=privacy_filter)

        # Create classified request with valid consent
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(
                raw_string="test",
                has_consent=True,
                purpose_consent={1, 2, 7, 9, 10},
            ),
        )
        request = ClassifiedRequest(
            impression_id="1",
            ad_format=AdFormat.BANNER,
            consent_signals=signals,
        )

        # Create scores
        scores = [
            BidderScore(bidder_code="appnexus", total_score=80, confidence=0.9),
            BidderScore(bidder_code="rubicon", total_score=75, confidence=0.9),
        ]

        result = selector.select_partners(scores, request)

        # Bidders should be selected
        assert len(result.selected) >= 2
        assert result.privacy_filtered_count == 0

    def test_selector_privacy_disabled(self):
        """Selector should skip privacy filter when disabled."""
        from src.idr.selector.partner_selector import PartnerSelector, SelectorConfig
        from src.idr.models.bidder_score import BidderScore
        from src.idr.models.classified_request import ClassifiedRequest, AdFormat

        privacy_filter = PrivacyFilter()
        config = SelectorConfig(privacy_enabled=False, max_bidders=10)
        selector = PartnerSelector(config=config, privacy_filter=privacy_filter)

        # Create classified request with GDPR but no consent
        signals = ConsentSignals(
            gdpr_applies=True,
            tcf=TCFConsent(raw_string="", has_consent=False),
        )
        request = ClassifiedRequest(
            impression_id="1",
            ad_format=AdFormat.BANNER,
            consent_signals=signals,
        )

        # Create scores
        scores = [
            BidderScore(bidder_code="appnexus", total_score=80, confidence=0.9),
        ]

        result = selector.select_partners(scores, request)

        # Should not filter because privacy is disabled
        assert result.privacy_filtered_count == 0
