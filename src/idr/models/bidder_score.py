"""Bidder Score model for IDR scoring output."""

from dataclasses import dataclass, field
from typing import Optional


@dataclass
class ScoreComponents:
    """
    Individual score components that make up the total bidder score.

    Each component is scored 0-100.
    """

    win_rate: float = 0.0       # 25% weight - How often bidder wins
    bid_rate: float = 0.0       # 20% weight - How often bidder responds
    cpm: float = 0.0            # 15% weight - Average bid value
    floor_clearance: float = 0.0  # 15% weight - Likelihood of clearing floor
    latency: float = 0.0        # 10% weight - Response time reliability
    recency: float = 0.0        # 10% weight - Recent performance trend
    id_match: float = 0.0       # 5% weight - User ID availability

    def to_dict(self) -> dict:
        """Convert to dictionary representation."""
        return {
            'win_rate': round(self.win_rate, 2),
            'bid_rate': round(self.bid_rate, 2),
            'cpm': round(self.cpm, 2),
            'floor_clearance': round(self.floor_clearance, 2),
            'latency': round(self.latency, 2),
            'recency': round(self.recency, 2),
            'id_match': round(self.id_match, 2),
        }


@dataclass
class RecentMetrics:
    """
    Recent performance metrics (last 24 hours) for recency scoring.
    """

    sample_size: int = 0
    win_rate: float = 0.0
    bid_rate: float = 0.0
    avg_cpm: float = 0.0
    historical_win_rate: float = 0.0  # For trend comparison
    historical_bid_rate: float = 0.0
    historical_avg_cpm: float = 0.0

    @property
    def win_rate_trend(self) -> float:
        """Calculate win rate trend vs historical."""
        if self.historical_win_rate <= 0:
            return 1.0
        return self.win_rate / self.historical_win_rate

    @property
    def bid_rate_trend(self) -> float:
        """Calculate bid rate trend vs historical."""
        if self.historical_bid_rate <= 0:
            return 1.0
        return self.bid_rate / self.historical_bid_rate

    @property
    def cpm_trend(self) -> float:
        """Calculate CPM trend vs historical."""
        if self.historical_avg_cpm <= 0:
            return 1.0
        return self.avg_cpm / self.historical_avg_cpm


@dataclass
class BidderScore:
    """
    Complete scoring output for a bidder.

    Contains the total weighted score, individual components,
    and confidence level based on data quality.
    """

    bidder_code: str
    total_score: float = 0.0  # 0-100 composite score
    components: ScoreComponents = field(default_factory=ScoreComponents)
    confidence: float = 0.0  # 0-1 confidence based on sample size

    # Metadata
    lookup_key_used: Optional[str] = None  # Which lookup key resolved
    fallback_level: int = 0  # 0 = exact match, higher = broader fallback

    def __lt__(self, other: 'BidderScore') -> bool:
        """Enable sorting by total score."""
        return self.total_score < other.total_score

    def __gt__(self, other: 'BidderScore') -> bool:
        """Enable sorting by total score."""
        return self.total_score > other.total_score

    @property
    def is_high_confidence(self) -> bool:
        """Check if score has high confidence (>0.7)."""
        return self.confidence >= 0.7

    @property
    def is_exploration_candidate(self) -> bool:
        """Check if bidder should be considered for exploration."""
        return self.confidence < 0.3

    @classmethod
    def zero(cls, bidder_code: str) -> 'BidderScore':
        """Create a zero score for a bidder."""
        return cls(
            bidder_code=bidder_code,
            total_score=0.0,
            components=ScoreComponents(),
            confidence=0.0,
        )

    def to_dict(self) -> dict:
        """Convert to dictionary representation."""
        return {
            'bidder_code': self.bidder_code,
            'total_score': round(self.total_score, 2),
            'components': self.components.to_dict(),
            'confidence': round(self.confidence, 3),
            'lookup_key_used': self.lookup_key_used,
            'fallback_level': self.fallback_level,
        }
