"""Tests for the Bidder Scorer."""

import pytest

from src.idr.models.bidder_metrics import BidderMetrics
from src.idr.models.bidder_score import RecentMetrics, ScoreComponents
from src.idr.models.classified_request import (
    AdFormat,
    ClassifiedRequest,
    DeviceType,
)
from src.idr.scorer import BidderScorer
from src.idr.scorer.bidder_scorer import MockPerformanceDB


class TestBidderScorer:
    """Test suite for BidderScorer."""

    @pytest.fixture
    def mock_db(self):
        """Create a mock performance database."""
        return MockPerformanceDB()

    @pytest.fixture
    def scorer(self, mock_db):
        """Create a scorer with mock DB."""
        return BidderScorer(performance_db=mock_db)

    @pytest.fixture
    def scorer_no_db(self):
        """Create a scorer without DB."""
        return BidderScorer()

    @pytest.fixture
    def sample_request(self):
        """Create a sample classified request."""
        return ClassifiedRequest(
            impression_id="test-imp-1",
            ad_format=AdFormat.BANNER,
            ad_sizes=["300x250"],
            device_type=DeviceType.MOBILE,
            country="GB",
            publisher_id="pub-123",
            user_ids={"uid2": "uid2-value", "id5": "id5-value"},
            floor_price=0.50,
        )

    @pytest.fixture
    def high_performing_metrics(self):
        """High-performing bidder metrics."""
        return BidderMetrics(
            bidder_code="rubicon",
            request_count=50000,
            bid_count=45000,  # 90% bid rate
            win_count=5000,  # 10% win rate (of requests)
            avg_cpm=3.50,
            floor_clearance_rate=0.95,
            p95_latency=80.0,  # Fast
            sample_size=50000,
        )

    @pytest.fixture
    def low_performing_metrics(self):
        """Low-performing bidder metrics."""
        return BidderMetrics(
            bidder_code="lowbidder",
            request_count=5000,
            bid_count=1500,  # 30% bid rate
            win_count=50,  # 1% win rate
            avg_cpm=0.80,
            floor_clearance_rate=0.40,
            p95_latency=450.0,  # Slow
            sample_size=5000,
        )

    def test_score_bidder_with_mock_db(self, scorer, sample_request):
        """Test scoring with mock database."""
        score = scorer.score_bidder("rubicon", sample_request)

        assert score.bidder_code == "rubicon"
        assert 0 <= score.total_score <= 100
        assert score.confidence > 0
        assert score.components is not None

    def test_score_bidder_without_db(self, scorer_no_db, sample_request):
        """Test scoring without database returns minimal metrics."""
        score = scorer_no_db.score_bidder("rubicon", sample_request)

        assert score.bidder_code == "rubicon"
        # Without DB, most metrics are 0, but latency (100 for 0ms) and
        # recency (50 default) and id_match still contribute some score
        assert score.confidence == 0.0
        # Verify that performance-based components are 0
        assert score.components.win_rate == 0.0
        assert score.components.bid_rate == 0.0
        assert score.components.cpm == 0.0

    def test_score_bidder_with_provided_metrics(
        self, scorer, sample_request, high_performing_metrics
    ):
        """Test scoring with explicitly provided metrics."""
        score = scorer.score_bidder(
            "rubicon", sample_request, metrics=high_performing_metrics
        )

        assert score.bidder_code == "rubicon"
        assert score.total_score > 70  # High performer should score well
        assert score.confidence >= 0.89  # High sample size (50k samples)

    def test_high_vs_low_performer_scoring(
        self, scorer, sample_request, high_performing_metrics, low_performing_metrics
    ):
        """Test that high performers score higher than low performers."""
        high_score = scorer.score_bidder(
            "rubicon", sample_request, metrics=high_performing_metrics
        )

        low_score = scorer.score_bidder(
            "lowbidder", sample_request, metrics=low_performing_metrics
        )

        assert high_score.total_score > low_score.total_score

    def test_score_all_bidders(self, scorer, sample_request):
        """Test scoring multiple bidders."""
        bidders = ["rubicon", "appnexus", "pubmatic", "openx", "triplelift"]

        scores = scorer.score_all_bidders(bidders, sample_request)

        assert len(scores) == 5
        # Should be sorted by total_score descending
        for i in range(len(scores) - 1):
            assert scores[i].total_score >= scores[i + 1].total_score

    def test_score_components_populated(
        self, scorer, sample_request, high_performing_metrics
    ):
        """Test that all score components are populated."""
        score = scorer.score_bidder(
            "rubicon", sample_request, metrics=high_performing_metrics
        )

        components = score.components
        assert isinstance(components, ScoreComponents)
        assert components.win_rate > 0
        assert components.bid_rate > 0
        assert components.cpm > 0
        assert components.floor_clearance > 0
        assert components.latency > 0


class TestScoringFormulas:
    """Test individual scoring formulas."""

    @pytest.fixture
    def scorer(self):
        return BidderScorer()

    def test_win_rate_scoring(self, scorer):
        """Test win rate scoring formula."""
        # 0% win rate = 0 score
        assert scorer._score_win_rate(0.0) == 0.0

        # 10% win rate = 50 score
        assert scorer._score_win_rate(0.10) == 50.0

        # 20% win rate = 100 score (capped)
        assert scorer._score_win_rate(0.20) == 100.0

        # 30% win rate = 100 (capped)
        assert scorer._score_win_rate(0.30) == 100.0

    def test_bid_rate_scoring(self, scorer):
        """Test bid rate scoring formula."""
        # 0% bid rate = 0 score
        assert scorer._score_bid_rate(0.0) == 0.0

        # 40% bid rate = 50 score
        assert scorer._score_bid_rate(0.40) == 50.0

        # 80% bid rate = 100 score
        assert scorer._score_bid_rate(0.80) == 100.0

        # 100% bid rate = 100 (capped)
        assert scorer._score_bid_rate(1.0) == 100.0

    def test_cpm_scoring_no_floor(self, scorer):
        """Test CPM scoring without floor price."""
        # £0 CPM = 0 score
        assert scorer._score_avg_cpm(0.0, None) == 0.0

        # £2.50 CPM = 50 score
        assert scorer._score_avg_cpm(2.50, None) == 50.0

        # £5.00 CPM = 100 score
        assert scorer._score_avg_cpm(5.0, None) == 100.0

    def test_cpm_scoring_with_floor(self, scorer):
        """Test CPM scoring with floor price."""
        floor = 1.0

        # CPM below floor = penalized
        assert scorer._score_avg_cpm(0.50, floor) < 50.0

        # CPM at floor = 50 score
        assert scorer._score_avg_cpm(1.0, floor) == 50.0

        # CPM above floor = rewarded
        assert scorer._score_avg_cpm(2.0, floor) > 50.0

    def test_floor_clearance_scoring(self, scorer):
        """Test floor clearance scoring formula."""
        assert scorer._score_floor_clearance(0.0) == 0.0
        assert scorer._score_floor_clearance(0.5) == 50.0
        assert scorer._score_floor_clearance(1.0) == 100.0

    def test_latency_scoring(self, scorer):
        """Test latency scoring formula."""
        # <100ms = 100 score
        assert scorer._score_latency(50.0) == 100.0
        assert scorer._score_latency(100.0) == 100.0

        # 300ms = 50 score
        assert scorer._score_latency(300.0) == 50.0

        # >=500ms = 0 score
        assert scorer._score_latency(500.0) == 0.0
        assert scorer._score_latency(600.0) == 0.0

    def test_recency_scoring(self, scorer):
        """Test recency scoring formula."""
        # Insufficient data = neutral
        low_sample = RecentMetrics(sample_size=50)
        assert scorer._score_recency(low_sample) == 50.0

        # Improving trend (2x) = 100
        improving = RecentMetrics(
            sample_size=1000,
            win_rate=0.20,
            historical_win_rate=0.10,
        )
        assert scorer._score_recency(improving) == 100.0

        # Stable trend (1x) = 50
        stable = RecentMetrics(
            sample_size=1000,
            win_rate=0.10,
            historical_win_rate=0.10,
        )
        assert scorer._score_recency(stable) == 50.0

        # Declining trend (0.5x) = 25
        declining = RecentMetrics(
            sample_size=1000,
            win_rate=0.05,
            historical_win_rate=0.10,
        )
        assert scorer._score_recency(declining) == 25.0

    def test_id_match_scoring(self, scorer):
        """Test ID match scoring formula."""
        # Rubicon prefers: liveramp, uid2, id5

        # All IDs available = 100
        all_ids = {"liveramp": "x", "uid2": "y", "id5": "z"}
        assert scorer._score_id_match("rubicon", all_ids) == 100.0

        # 2 of 3 IDs = ~66.7
        partial_ids = {"uid2": "y", "id5": "z"}
        score = scorer._score_id_match("rubicon", partial_ids)
        assert 65 <= score <= 68

        # No IDs = 0
        assert scorer._score_id_match("rubicon", {}) == 0.0


class TestScoringWeights:
    """Test scoring weight configuration."""

    def test_default_weights_sum_to_one(self):
        """Test that default weights sum to 1.0."""
        scorer = BidderScorer()
        assert abs(sum(scorer.weights.values()) - 1.0) < 0.001

    def test_custom_weights(self):
        """Test custom weight configuration."""
        custom_weights = {
            "win_rate": 0.30,
            "bid_rate": 0.30,
            "cpm": 0.20,
            "floor_clearance": 0.10,
            "latency": 0.05,
            "recency": 0.03,
            "id_match": 0.02,
        }

        scorer = BidderScorer(weights=custom_weights)
        assert scorer.weights["win_rate"] == 0.30

    def test_invalid_weights_raise_error(self):
        """Test that weights not summing to 1.0 raise error."""
        invalid_weights = {
            "win_rate": 0.50,
            "bid_rate": 0.50,
            "cpm": 0.50,  # Total > 1.0
            "floor_clearance": 0.0,
            "latency": 0.0,
            "recency": 0.0,
            "id_match": 0.0,
        }

        with pytest.raises(ValueError, match="must sum to 1.0"):
            BidderScorer(weights=invalid_weights)


class TestBidderMetrics:
    """Test BidderMetrics model."""

    def test_computed_rates(self):
        """Test that rates are computed from counts."""
        metrics = BidderMetrics(
            bidder_code="test",
            request_count=1000,
            bid_count=700,
            win_count=100,
        )

        assert metrics.bid_rate == 0.7
        # win_rate is win_count / bid_count
        assert abs(metrics.win_rate - (100 / 700)) < 0.001

    def test_sample_size_confidence(self):
        """Test confidence calculation based on sample size."""
        # Low samples = low confidence
        low_samples = BidderMetrics(bidder_code="test", sample_size=50)
        assert low_samples.sample_size_confidence < 0.3

        # Medium samples
        medium_samples = BidderMetrics(bidder_code="test", sample_size=1000)
        assert 0.3 < medium_samples.sample_size_confidence < 0.7

        # High samples = high confidence
        high_samples = BidderMetrics(bidder_code="test", sample_size=100000)
        assert high_samples.sample_size_confidence == 1.0

    def test_empty_metrics(self):
        """Test creating empty metrics."""
        empty = BidderMetrics.empty("nobidder")

        assert empty.bidder_code == "nobidder"
        assert empty.request_count == 0
        assert empty.sample_size_confidence == 0.0


class TestBidderScore:
    """Test BidderScore model."""

    def test_score_comparison(self):
        """Test score comparison operators."""
        from src.idr.models.bidder_score import BidderScore

        high = BidderScore(bidder_code="high", total_score=80.0)
        low = BidderScore(bidder_code="low", total_score=40.0)

        assert high > low
        assert low < high

    def test_is_high_confidence(self):
        """Test high confidence detection."""
        from src.idr.models.bidder_score import BidderScore

        high_conf = BidderScore(bidder_code="x", confidence=0.9)
        low_conf = BidderScore(bidder_code="y", confidence=0.5)

        assert high_conf.is_high_confidence is True
        assert low_conf.is_high_confidence is False

    def test_is_exploration_candidate(self):
        """Test exploration candidate detection."""
        from src.idr.models.bidder_score import BidderScore

        explore = BidderScore(bidder_code="x", confidence=0.2)
        no_explore = BidderScore(bidder_code="y", confidence=0.5)

        assert explore.is_exploration_candidate is True
        assert no_explore.is_exploration_candidate is False

    def test_to_dict(self):
        """Test conversion to dictionary."""
        from src.idr.models.bidder_score import BidderScore

        score = BidderScore(
            bidder_code="rubicon",
            total_score=75.5,
            confidence=0.85,
        )
        result = score.to_dict()

        assert result["bidder_code"] == "rubicon"
        assert result["total_score"] == 75.5
        assert result["confidence"] == 0.85
