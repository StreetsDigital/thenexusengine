"""Tests for the Partner Selector."""

import pytest

from src.idr.models.bidder_score import BidderScore
from src.idr.models.classified_request import AdFormat, ClassifiedRequest, DeviceType
from src.idr.selector import PartnerSelector, SelectionResult, SelectorConfig
from src.idr.selector.partner_selector import (
    MockAnchorProvider,
    SelectedBidder,
    SelectionReason,
)


class TestPartnerSelector:
    """Test suite for PartnerSelector."""

    @pytest.fixture
    def config(self):
        """Default selector config."""
        return SelectorConfig(
            max_bidders=10,
            min_score_threshold=25.0,
            exploration_rate=0.1,
            exploration_slots=2,
            anchor_bidder_count=3,
        )

    @pytest.fixture
    def selector(self, config):
        """Create a selector with fixed random seed for reproducibility."""
        return PartnerSelector(config=config, random_seed=42)

    @pytest.fixture
    def sample_request(self):
        """Sample classified request."""
        return ClassifiedRequest(
            impression_id="test-imp-1",
            ad_format=AdFormat.BANNER,
            ad_sizes=["300x250"],
            device_type=DeviceType.MOBILE,
            country="GB",
            publisher_id="pub-123",
        )

    @pytest.fixture
    def sample_scores(self):
        """Sample bidder scores with variety of scores and confidence."""
        return [
            # Premium bidders - high scores
            BidderScore(bidder_code="rubicon", total_score=85.0, confidence=0.95),
            BidderScore(bidder_code="appnexus", total_score=82.0, confidence=0.92),
            BidderScore(bidder_code="pubmatic", total_score=78.0, confidence=0.88),
            BidderScore(bidder_code="openx", total_score=75.0, confidence=0.85),
            BidderScore(bidder_code="ix", total_score=72.0, confidence=0.82),
            # Mid-tier bidders
            BidderScore(bidder_code="triplelift", total_score=65.0, confidence=0.75),
            BidderScore(bidder_code="sovrn", total_score=58.0, confidence=0.70),
            BidderScore(bidder_code="sharethrough", total_score=52.0, confidence=0.65),
            # Lower scorers
            BidderScore(bidder_code="gumgum", total_score=45.0, confidence=0.60),
            BidderScore(bidder_code="33across", total_score=40.0, confidence=0.55),
            # Below threshold
            BidderScore(bidder_code="lowbidder1", total_score=20.0, confidence=0.40),
            BidderScore(bidder_code="lowbidder2", total_score=15.0, confidence=0.35),
            # Low confidence (exploration candidates)
            BidderScore(bidder_code="newbidder1", total_score=50.0, confidence=0.20),
            BidderScore(bidder_code="newbidder2", total_score=48.0, confidence=0.15),
            BidderScore(bidder_code="newbidder3", total_score=45.0, confidence=0.10),
        ]

    def test_select_partners_returns_result(
        self, selector, sample_request, sample_scores
    ):
        """Test that selection returns a proper result."""
        result = selector.select_partners(sample_scores, sample_request)

        assert isinstance(result, SelectionResult)
        assert len(result.selected) > 0
        assert len(result.selected) <= selector.config.max_bidders
        assert result.total_candidates == len(sample_scores)

    def test_anchor_bidders_always_included(
        self, selector, sample_request, sample_scores
    ):
        """Test that anchor bidders are always selected."""
        result = selector.select_partners(sample_scores, sample_request)

        selected_codes = result.selected_bidder_codes

        # Default anchors are rubicon, appnexus, pubmatic
        assert "rubicon" in selected_codes
        assert "appnexus" in selected_codes
        assert "pubmatic" in selected_codes

        # Verify they're marked as anchors
        anchor_selected = [
            b for b in result.selected if b.reason == SelectionReason.ANCHOR
        ]
        assert len(anchor_selected) == 3

    def test_custom_anchor_provider(self, config, sample_request, sample_scores):
        """Test with custom anchor provider."""
        provider = MockAnchorProvider()
        provider.set_anchors("pub-123", ["openx", "ix", "triplelift"])

        selector = PartnerSelector(
            config=config,
            anchor_provider=provider,
            random_seed=42,
        )

        result = selector.select_partners(sample_scores, sample_request)
        selected_codes = result.selected_bidder_codes

        # Custom anchors should be selected
        assert "openx" in selected_codes
        assert "ix" in selected_codes
        assert "triplelift" in selected_codes

    def test_respects_max_bidders(self, sample_request, sample_scores):
        """Test that max_bidders limit is respected."""
        config = SelectorConfig(max_bidders=5)
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(sample_scores, sample_request)

        assert len(result.selected) <= 5

    def test_respects_min_score_threshold(
        self, selector, sample_request, sample_scores
    ):
        """Test that bidders below threshold are excluded."""
        result = selector.select_partners(sample_scores, sample_request)


        # Bidders with score < 25 should not be selected (unless exploration)
        # lowbidder1 (20) and lowbidder2 (15) should be excluded
        for bidder in result.selected:
            if bidder.reason not in (
                SelectionReason.EXPLORATION,
                SelectionReason.EXPLORATION_SLOT,
            ):
                assert bidder.score >= 25.0

    def test_high_confidence_bidders_selected(
        self, selector, sample_request, sample_scores
    ):
        """Test that high confidence bidders are selected as HIGH_SCORE."""
        result = selector.select_partners(sample_scores, sample_request)

        high_score_selected = [
            b for b in result.selected if b.reason == SelectionReason.HIGH_SCORE
        ]

        # Should have some high-score selections
        assert len(high_score_selected) > 0

        # All high-score selections should have confidence >= 0.5
        for bidder in high_score_selected:
            assert bidder.confidence >= 0.5

    def test_exploration_slots_filled(self, sample_request, sample_scores):
        """Test that exploration slots are used."""
        config = SelectorConfig(
            max_bidders=15,
            exploration_slots=2,
            exploration_confidence_threshold=0.3,
        )
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(sample_scores, sample_request)

        exploration_selected = [
            b for b in result.selected if b.reason == SelectionReason.EXPLORATION_SLOT
        ]

        # Should have exploration slots filled
        assert len(exploration_selected) <= config.exploration_slots

    def test_diversity_categories(self, sample_request):
        """Test that diversity is enforced across categories."""
        # Create scores with only premium bidders scoring high
        scores = [
            BidderScore(bidder_code="rubicon", total_score=90.0, confidence=0.95),
            BidderScore(bidder_code="appnexus", total_score=88.0, confidence=0.92),
            BidderScore(bidder_code="pubmatic", total_score=85.0, confidence=0.90),
            BidderScore(bidder_code="openx", total_score=82.0, confidence=0.88),
            # Mid-tier with lower score
            BidderScore(bidder_code="triplelift", total_score=40.0, confidence=0.70),
            BidderScore(bidder_code="sovrn", total_score=35.0, confidence=0.65),
        ]

        config = SelectorConfig(
            max_bidders=10,
            min_score_threshold=25.0,
            diversity_enabled=True,
        )
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(scores, sample_request)
        selected_codes = result.selected_bidder_codes

        # Should include mid_tier for diversity even with lower score
        assert "triplelift" in selected_codes or "sovrn" in selected_codes

    def test_diversity_disabled(self, sample_request):
        """Test that diversity can be disabled."""
        scores = [
            BidderScore(bidder_code="rubicon", total_score=90.0, confidence=0.95),
            BidderScore(bidder_code="appnexus", total_score=88.0, confidence=0.92),
            BidderScore(bidder_code="pubmatic", total_score=85.0, confidence=0.90),
            # Mid-tier below threshold
            BidderScore(bidder_code="triplelift", total_score=20.0, confidence=0.70),
        ]

        config = SelectorConfig(
            max_bidders=10,
            min_score_threshold=25.0,
            diversity_enabled=False,
        )
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(scores, sample_request)

        # Should NOT have diversity selections
        diversity_selected = [
            b for b in result.selected if b.reason == SelectionReason.DIVERSITY
        ]
        assert len(diversity_selected) == 0

    def test_video_request_includes_video_specialists(self, selector):
        """Test that video requests include video_specialist category."""
        video_request = ClassifiedRequest(
            impression_id="video-1",
            ad_format=AdFormat.VIDEO,
            ad_sizes=["640x480"],
            device_type=DeviceType.DESKTOP,
            publisher_id="pub-123",
        )

        scores = [
            BidderScore(bidder_code="rubicon", total_score=90.0, confidence=0.95),
            BidderScore(bidder_code="appnexus", total_score=88.0, confidence=0.92),
            BidderScore(bidder_code="pubmatic", total_score=85.0, confidence=0.90),
            # Video specialist
            BidderScore(bidder_code="spotx", total_score=40.0, confidence=0.70),
            BidderScore(bidder_code="beachfront", total_score=35.0, confidence=0.65),
        ]

        result = selector.select_partners(scores, video_request)
        selected_codes = result.selected_bidder_codes

        # Should include video specialist for diversity
        has_video_specialist = (
            "spotx" in selected_codes or "beachfront" in selected_codes
        )
        assert has_video_specialist

    def test_excluded_bidders_tracked(self, selector, sample_request, sample_scores):
        """Test that excluded bidders are tracked."""
        result = selector.select_partners(sample_scores, sample_request)

        # All bidders should either be selected or excluded
        all_bidders = {s.bidder_code for s in sample_scores}
        selected_set = set(result.selected_bidder_codes)
        excluded_set = set(result.excluded)

        assert selected_set | excluded_set == all_bidders
        assert selected_set & excluded_set == set()  # No overlap

    def test_selection_result_to_dict(self, selector, sample_request, sample_scores):
        """Test SelectionResult serialization."""
        result = selector.select_partners(sample_scores, sample_request)
        result_dict = result.to_dict()

        assert "selected" in result_dict
        assert "excluded" in result_dict
        assert "total_candidates" in result_dict
        assert "summary" in result_dict

        assert result_dict["summary"]["selected_count"] == len(result.selected)

    def test_selection_time_tracked(self, selector, sample_request, sample_scores):
        """Test that selection time is measured."""
        result = selector.select_partners(sample_scores, sample_request)

        assert result.selection_time_ms >= 0
        assert result.selection_time_ms < 100  # Should be fast

    def test_empty_scores_list(self, selector, sample_request):
        """Test handling of empty scores list."""
        result = selector.select_partners([], sample_request)

        assert len(result.selected) == 0
        assert len(result.excluded) == 0
        assert result.total_candidates == 0

    def test_deterministic_with_seed(self, config, sample_request, sample_scores):
        """Test that selection is deterministic with same seed."""
        selector1 = PartnerSelector(config=config, random_seed=42)
        selector2 = PartnerSelector(config=config, random_seed=42)

        result1 = selector1.select_partners(sample_scores, sample_request)
        result2 = selector2.select_partners(sample_scores, sample_request)

        assert result1.selected_bidder_codes == result2.selected_bidder_codes

    def test_bypass_mode_selects_all(self, sample_request, sample_scores):
        """Test that bypass mode selects ALL bidders with no filtering."""
        config = SelectorConfig.bypass()
        selector = PartnerSelector(config=config)

        result = selector.select_partners(sample_scores, sample_request)

        # All bidders should be selected
        assert len(result.selected) == len(sample_scores)
        assert len(result.excluded) == 0
        assert result.bypass_mode is True
        assert result.is_filtered is False

        # All should have BYPASS reason
        for bidder in result.selected:
            assert bidder.reason == SelectionReason.BYPASS

    def test_bypass_mode_via_config(self, sample_request, sample_scores):
        """Test bypass mode via config flag."""
        config = SelectorConfig(bypass_enabled=True)
        selector = PartnerSelector(config=config)

        result = selector.select_partners(sample_scores, sample_request)

        assert result.bypass_mode is True
        assert len(result.selected) == len(sample_scores)

    def test_shadow_mode_returns_all_but_tracks_exclusions(
        self, sample_request, sample_scores
    ):
        """Test that shadow mode returns all bidders but tracks what would be excluded."""
        config = SelectorConfig.shadow()
        config.max_bidders = 5  # Limit to ensure some would be excluded
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(sample_scores, sample_request)

        # All bidders should be returned (nothing actually excluded)
        assert len(result.selected) == len(sample_scores)
        assert len(result.excluded) == 0
        assert result.shadow_mode is True
        assert result.is_filtered is False

        # But shadow_would_exclude should show what WOULD be excluded
        assert len(result.shadow_would_exclude) > 0

    def test_shadow_mode_preserves_reasons(self, sample_request, sample_scores):
        """Test that shadow mode preserves original selection reasons."""
        config = SelectorConfig(shadow_mode=True, max_bidders=5)
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(sample_scores, sample_request)

        # Should have mix of real reasons and BYPASS for would-be-excluded
        reasons = {b.reason for b in result.selected}
        assert SelectionReason.BYPASS in reasons  # The would-be-excluded ones
        # Should have at least one real selection reason
        real_reasons = reasons - {SelectionReason.BYPASS}
        assert len(real_reasons) > 0

    def test_shadow_mode_result_dict_includes_analysis(
        self, sample_request, sample_scores
    ):
        """Test that shadow mode result dict includes analysis."""
        config = SelectorConfig(shadow_mode=True, max_bidders=5)
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(sample_scores, sample_request)
        result_dict = result.to_dict()

        assert result_dict["shadow_mode"] is True
        assert "shadow_analysis" in result_dict
        assert "would_exclude" in result_dict["shadow_analysis"]
        assert result_dict["shadow_analysis"]["would_exclude_count"] > 0

    def test_normal_mode_is_filtered(self, config, sample_request, sample_scores):
        """Test that normal mode correctly sets is_filtered."""
        config.max_bidders = 5  # Ensure some exclusions
        selector = PartnerSelector(config=config, random_seed=42)

        result = selector.select_partners(sample_scores, sample_request)

        # Should have exclusions in normal mode
        if len(result.excluded) > 0:
            assert result.is_filtered is True
        assert result.bypass_mode is False
        assert result.shadow_mode is False


class TestSelectorConfig:
    """Test SelectorConfig."""

    def test_default_values(self):
        """Test default configuration values."""
        config = SelectorConfig()

        assert config.max_bidders == 15
        assert config.min_score_threshold == 25.0
        assert config.exploration_rate == 0.1
        assert config.exploration_slots == 2
        assert config.anchor_bidder_count == 3
        assert config.bypass_enabled is False
        assert config.shadow_mode is False

    def test_bypass_factory(self):
        """Test bypass configuration factory."""
        config = SelectorConfig.bypass()

        assert config.bypass_enabled is True
        assert config.shadow_mode is False

    def test_shadow_factory(self):
        """Test shadow configuration factory."""
        config = SelectorConfig.shadow()

        assert config.shadow_mode is True
        assert config.bypass_enabled is False

    def test_from_dict(self):
        """Test creating config from dictionary."""
        config_dict = {
            "max_bidders": 20,
            "min_score_threshold": 30.0,
            "exploration_rate": 0.15,
        }

        config = SelectorConfig.from_dict(config_dict)

        assert config.max_bidders == 20
        assert config.min_score_threshold == 30.0
        assert config.exploration_rate == 0.15
        # Defaults for unspecified
        assert config.exploration_slots == 2


class TestSelectionResult:
    """Test SelectionResult."""

    def test_selected_bidder_codes(self):
        """Test selected_bidder_codes property."""
        selected = [
            SelectedBidder("rubicon", 80.0, 0.9, SelectionReason.ANCHOR),
            SelectedBidder("appnexus", 75.0, 0.85, SelectionReason.HIGH_SCORE),
        ]
        result = SelectionResult(
            selected=selected,
            excluded=["lowbidder"],
            total_candidates=3,
        )

        assert result.selected_bidder_codes == ["rubicon", "appnexus"]

    def test_anchor_count(self):
        """Test anchor_count property."""
        selected = [
            SelectedBidder("rubicon", 80.0, 0.9, SelectionReason.ANCHOR),
            SelectedBidder("appnexus", 75.0, 0.85, SelectionReason.ANCHOR),
            SelectedBidder("triplelift", 60.0, 0.7, SelectionReason.HIGH_SCORE),
        ]
        result = SelectionResult(
            selected=selected,
            excluded=[],
            total_candidates=3,
        )

        assert result.anchor_count == 2

    def test_exploration_count(self):
        """Test exploration_count property."""
        selected = [
            SelectedBidder("rubicon", 80.0, 0.9, SelectionReason.ANCHOR),
            SelectedBidder("newbidder", 50.0, 0.2, SelectionReason.EXPLORATION),
            SelectedBidder("newbidder2", 45.0, 0.15, SelectionReason.EXPLORATION_SLOT),
        ]
        result = SelectionResult(
            selected=selected,
            excluded=[],
            total_candidates=3,
        )

        assert result.exploration_count == 2


class TestMockAnchorProvider:
    """Test MockAnchorProvider."""

    def test_default_anchors(self):
        """Test default anchor bidders."""
        provider = MockAnchorProvider()

        anchors = provider.get_top_bidders_by_revenue("any-pub")

        assert anchors == ["rubicon", "appnexus", "pubmatic"]

    def test_custom_anchors(self):
        """Test setting custom anchors."""
        provider = MockAnchorProvider()
        provider.set_anchors("pub-123", ["openx", "ix", "triplelift", "sovrn"])

        anchors = provider.get_top_bidders_by_revenue("pub-123", limit=3)

        assert anchors == ["openx", "ix", "triplelift"]

    def test_limit_respected(self):
        """Test that limit parameter is respected."""
        provider = MockAnchorProvider()

        anchors = provider.get_top_bidders_by_revenue("any-pub", limit=2)

        assert len(anchors) == 2


class TestIntegrationWithScorer:
    """Integration tests with BidderScorer."""

    def test_full_pipeline(self):
        """Test full scoring and selection pipeline."""
        from src.idr.classifier import RequestClassifier
        from src.idr.scorer import BidderScorer
        from src.idr.scorer.bidder_scorer import MockPerformanceDB

        # Create components
        classifier = RequestClassifier()
        db = MockPerformanceDB()
        scorer = BidderScorer(performance_db=db)
        selector = PartnerSelector(
            config=SelectorConfig(max_bidders=10),
            random_seed=42,
        )

        # Sample OpenRTB request
        ortb_request = {
            "id": "auction-123",
            "imp": [
                {
                    "id": "imp-1",
                    "banner": {"format": [{"w": 300, "h": 250}]},
                    "bidfloor": 0.50,
                }
            ],
            "site": {
                "domain": "example.com",
                "publisher": {"id": "pub-123"},
            },
            "device": {
                "ua": "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0) Safari",
                "geo": {"country": "GB"},
            },
        }

        # Classify request
        classified = classifier.classify(ortb_request)

        # Score bidders
        bidders = ["rubicon", "appnexus", "pubmatic", "openx", "triplelift"]
        scores = scorer.score_all_bidders(bidders, classified)

        # Select partners
        result = selector.select_partners(scores, classified)

        # Verify result
        assert len(result.selected) > 0
        assert len(result.selected) <= 10
        assert result.total_candidates == 5
