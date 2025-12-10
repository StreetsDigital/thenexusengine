"""
Bidder Scorer for the Intelligent Demand Router (IDR).

Scores each potential bidder (0-100) based on likelihood of responding
with a competitive bid for the given request.
"""

from typing import Optional, Protocol

from ..models.bidder_metrics import BidderMetrics
from ..models.bidder_score import BidderScore, RecentMetrics, ScoreComponents
from ..models.classified_request import ClassifiedRequest
from ..models.lookup_key import LookupKey
from ..utils.constants import (
    DEFAULT_ID_PREFERENCES,
    ID_PREFERENCES,
    MIN_SAMPLE_SIZE,
    SCORING_WEIGHTS,
)


class PerformanceDB(Protocol):
    """Protocol for performance database interface."""

    def get_bidder_metrics(
        self,
        bidder_code: str,
        lookup_key: LookupKey
    ) -> BidderMetrics:
        """Get historical metrics for a bidder."""
        ...

    def get_recent_metrics(
        self,
        bidder_code: str,
        lookup_key: LookupKey,
        hours: int = 24
    ) -> RecentMetrics:
        """Get recent metrics for trend analysis."""
        ...


class BidderScorer:
    """
    Scores bidders based on historical performance and request attributes.

    Scoring dimensions (weights defined in constants):
    - Historical Win Rate (25%): How often this bidder wins for similar requests
    - Bid Rate (20%): How often this bidder responds at all
    - Avg Bid CPM (15%): Average bid value for similar inventory
    - Floor Clearance (15%): Likelihood of clearing the floor price
    - Latency Score (10%): Response time reliability
    - Recency (10%): Recent performance (last 24h weighted higher)
    - ID Match Rate (5%): User ID availability for this bidder
    """

    def __init__(
        self,
        performance_db: Optional[PerformanceDB] = None,
        weights: Optional[dict[str, float]] = None,
    ):
        """
        Initialize the bidder scorer.

        Args:
            performance_db: Database interface for historical metrics
            weights: Custom scoring weights (optional, uses defaults)
        """
        self.db = performance_db
        self.weights = weights or SCORING_WEIGHTS

        # Validate weights sum to 1.0
        weight_sum = sum(self.weights.values())
        if abs(weight_sum - 1.0) > 0.001:
            raise ValueError(
                f"Scoring weights must sum to 1.0, got {weight_sum}"
            )

    def score_bidder(
        self,
        bidder_code: str,
        request: ClassifiedRequest,
        metrics: Optional[BidderMetrics] = None,
        recent_metrics: Optional[RecentMetrics] = None,
    ) -> BidderScore:
        """
        Score a bidder for the given classified request.

        Args:
            bidder_code: The bidder's identifier
            request: Classified bid request
            metrics: Pre-fetched metrics (optional, will query DB if not provided)
            recent_metrics: Pre-fetched recent metrics (optional)

        Returns:
            BidderScore with total score and component breakdown
        """
        # Build lookup key for historical data
        lookup_key = self._build_lookup_key(request)
        fallback_level = 0

        # Fetch metrics if not provided
        if metrics is None:
            if self.db:
                metrics, fallback_level = self._fetch_metrics_with_fallback(
                    bidder_code, lookup_key
                )
            else:
                metrics = BidderMetrics.empty(bidder_code)

        # Fetch recent metrics if not provided
        if recent_metrics is None:
            if self.db:
                recent_metrics = self.db.get_recent_metrics(
                    bidder_code, lookup_key
                )
            else:
                recent_metrics = RecentMetrics()

        # Calculate component scores (0-100 each)
        components = ScoreComponents(
            win_rate=self._score_win_rate(metrics.win_rate),
            bid_rate=self._score_bid_rate(metrics.bid_rate),
            cpm=self._score_avg_cpm(metrics.avg_cpm, request.floor_price),
            floor_clearance=self._score_floor_clearance(
                metrics.floor_clearance_rate
            ),
            latency=self._score_latency(metrics.p95_latency),
            recency=self._score_recency(recent_metrics),
            id_match=self._score_id_match(bidder_code, request.user_ids),
        )

        # Calculate weighted total score
        total_score = (
            components.win_rate * self.weights['win_rate'] +
            components.bid_rate * self.weights['bid_rate'] +
            components.cpm * self.weights['cpm'] +
            components.floor_clearance * self.weights['floor_clearance'] +
            components.latency * self.weights['latency'] +
            components.recency * self.weights['recency'] +
            components.id_match * self.weights['id_match']
        )

        return BidderScore(
            bidder_code=bidder_code,
            total_score=total_score,
            components=components,
            confidence=metrics.sample_size_confidence,
            lookup_key_used=str(lookup_key),
            fallback_level=fallback_level,
        )

    def score_all_bidders(
        self,
        bidder_codes: list[str],
        request: ClassifiedRequest,
    ) -> list[BidderScore]:
        """
        Score multiple bidders for the given request.

        Args:
            bidder_codes: List of bidder identifiers to score
            request: Classified bid request

        Returns:
            List of BidderScore objects sorted by total_score descending
        """
        scores = [
            self.score_bidder(bidder_code, request)
            for bidder_code in bidder_codes
        ]

        # Sort by total score descending
        scores.sort(key=lambda s: s.total_score, reverse=True)

        return scores

    def _build_lookup_key(self, request: ClassifiedRequest) -> LookupKey:
        """
        Build a lookup key for historical data retrieval.

        Creates a hierarchical key with fallback support:
        1. Exact match: country + device + format + size
        2. Broader: country + device + format
        3. Broadest: country + device
        """
        return LookupKey(
            country=request.country,
            device_type=request.device_type.value,
            ad_format=request.ad_format.value,
            ad_size=request.primary_ad_size,
            publisher_id=request.publisher_id,
        )

    def _fetch_metrics_with_fallback(
        self,
        bidder_code: str,
        lookup_key: LookupKey,
    ) -> tuple[BidderMetrics, int]:
        """
        Fetch metrics with hierarchical fallback.

        Returns:
            Tuple of (metrics, fallback_level)
        """
        fallback_keys = lookup_key.get_fallback_keys()

        for level, key in enumerate(fallback_keys):
            metrics = self.db.get_bidder_metrics(bidder_code, key)
            if metrics.sample_size >= MIN_SAMPLE_SIZE:
                return (metrics, level)

        # Return last (broadest) metrics even if insufficient samples
        return (
            self.db.get_bidder_metrics(bidder_code, fallback_keys[-1]),
            len(fallback_keys) - 1
        )

    def _score_win_rate(self, win_rate: float) -> float:
        """
        Score based on historical win rate.

        20%+ win rate = 100 score
        0% win rate = 0 score
        Linear scale between.

        Args:
            win_rate: Win rate as decimal (0.0 to 1.0)

        Returns:
            Score from 0 to 100
        """
        # 20% (0.20) win rate maps to 100
        return min(100.0, win_rate * 500.0)

    def _score_bid_rate(self, bid_rate: float) -> float:
        """
        Score based on bid response rate.

        80%+ bid rate = 100 score
        0% bid rate = 0 score
        Linear scale between.

        Args:
            bid_rate: Bid rate as decimal (0.0 to 1.0)

        Returns:
            Score from 0 to 100
        """
        # 80% (0.80) bid rate maps to 100
        return min(100.0, bid_rate * 125.0)

    def _score_avg_cpm(
        self,
        avg_cpm: float,
        floor_price: Optional[float]
    ) -> float:
        """
        Score based on average CPM relative to floor.

        If no floor:
            Score based on absolute CPM (£5 CPM = 100)
        If floor exists:
            Score based on headroom above floor

        Args:
            avg_cpm: Average CPM in currency units
            floor_price: Floor price (optional)

        Returns:
            Score from 0 to 100
        """
        if floor_price is None or floor_price <= 0:
            # No floor - score based on absolute CPM
            # £5 CPM = 100, linear scale
            return min(100.0, avg_cpm * 20.0)

        # Score based on headroom above floor
        if avg_cpm < floor_price:
            # Below floor - penalize but not to zero
            ratio = avg_cpm / floor_price
            return max(0.0, 50.0 * ratio)

        # Above floor - reward headroom
        headroom = (avg_cpm - floor_price) / floor_price
        return min(100.0, max(0.0, 50.0 + headroom * 100.0))

    def _score_floor_clearance(self, clearance_rate: float) -> float:
        """
        Score based on floor price clearance rate.

        100% clearance = 100 score
        0% clearance = 0 score
        Linear scale.

        Args:
            clearance_rate: Floor clearance rate as decimal (0.0 to 1.0)

        Returns:
            Score from 0 to 100
        """
        return clearance_rate * 100.0

    def _score_latency(self, p95_latency_ms: float) -> float:
        """
        Score based on p95 response latency.

        <100ms = 100 score
        >500ms = 0 score
        Linear scale between.

        Args:
            p95_latency_ms: 95th percentile latency in milliseconds

        Returns:
            Score from 0 to 100
        """
        if p95_latency_ms <= 100:
            return 100.0
        if p95_latency_ms >= 500:
            return 0.0

        # Linear interpolation from 100ms to 500ms
        # 100ms -> 100, 500ms -> 0
        return 100.0 - ((p95_latency_ms - 100.0) / 4.0)

    def _score_recency(self, recent_metrics: RecentMetrics) -> float:
        """
        Score based on recent performance trend.

        Compares last 24h performance against historical baseline.
        Trend > 1.0 = improving = higher score
        Trend < 1.0 = declining = lower score

        Args:
            recent_metrics: Recent (24h) performance metrics

        Returns:
            Score from 0 to 100
        """
        if recent_metrics.sample_size < MIN_SAMPLE_SIZE:
            # Not enough recent data - return neutral score
            return 50.0

        # Use win rate trend as primary signal
        trend = recent_metrics.win_rate_trend

        # Clamp trend to reasonable bounds (0.5x to 2x)
        trend = max(0.5, min(2.0, trend))

        # Map trend to score: 0.5 -> 25, 1.0 -> 50, 2.0 -> 100
        return min(100.0, max(0.0, 50.0 * trend))

    def _score_id_match(
        self,
        bidder_code: str,
        user_ids: dict[str, str]
    ) -> float:
        """
        Score based on availability of preferred user IDs.

        Bidders may prefer certain ID types. Score is based on
        how many of their preferred IDs are available.

        Args:
            bidder_code: The bidder's identifier
            user_ids: Dict of available user IDs {type: value}

        Returns:
            Score from 0 to 100
        """
        # Get bidder's ID preferences
        preferred_ids = ID_PREFERENCES.get(
            bidder_code,
            DEFAULT_ID_PREFERENCES
        )

        if not preferred_ids:
            # No preferences - return neutral
            return 50.0

        if not user_ids:
            # No user IDs available
            return 0.0

        # Count matches
        matches = sum(
            1 for id_type in preferred_ids
            if id_type in user_ids
        )

        # Score based on match ratio
        match_ratio = matches / len(preferred_ids)
        return match_ratio * 100.0


class MockPerformanceDB:
    """
    Mock performance database for testing and development.

    Returns reasonable default metrics.
    """

    def __init__(self, default_sample_size: int = 10000):
        """Initialize with default sample size."""
        self.default_sample_size = default_sample_size
        self._bidder_data: dict[str, dict] = {}

    def set_bidder_data(
        self,
        bidder_code: str,
        metrics: BidderMetrics
    ) -> None:
        """Set custom metrics for a bidder."""
        self._bidder_data[bidder_code] = metrics

    def get_bidder_metrics(
        self,
        bidder_code: str,
        lookup_key: LookupKey
    ) -> BidderMetrics:
        """Get mock metrics for a bidder."""
        if bidder_code in self._bidder_data:
            return self._bidder_data[bidder_code]

        # Return default reasonable metrics
        return BidderMetrics(
            bidder_code=bidder_code,
            request_count=self.default_sample_size,
            bid_count=int(self.default_sample_size * 0.7),  # 70% bid rate
            win_count=int(self.default_sample_size * 0.1),  # 10% win rate
            avg_cpm=2.50,
            floor_clearance_rate=0.85,
            p95_latency=150.0,
            sample_size=self.default_sample_size,
        )

    def get_recent_metrics(
        self,
        bidder_code: str,
        lookup_key: LookupKey,
        hours: int = 24
    ) -> RecentMetrics:
        """Get mock recent metrics."""
        return RecentMetrics(
            sample_size=int(self.default_sample_size / 7),
            win_rate=0.12,  # Slightly above historical
            bid_rate=0.72,
            avg_cpm=2.60,
            historical_win_rate=0.10,
            historical_bid_rate=0.70,
            historical_avg_cpm=2.50,
        )
