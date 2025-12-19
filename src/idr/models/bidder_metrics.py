"""Bidder Metrics model for historical performance data."""

from dataclasses import dataclass


@dataclass
class BidderMetrics:
    """
    Historical performance metrics for a bidder.

    Retrieved from the performance database based on lookup key.
    """

    bidder_code: str

    # Volume metrics
    request_count: int = 0
    bid_count: int = 0
    win_count: int = 0

    # Rate metrics (computed)
    bid_rate: float = 0.0  # bid_count / request_count
    win_rate: float = 0.0  # win_count / bid_count (of bids placed)

    # CPM metrics
    avg_cpm: float = 0.0
    max_cpm: float = 0.0
    min_cpm: float = 0.0

    # Floor metrics
    floor_clearance_rate: float = 0.0  # Rate of bids clearing floor price

    # Latency metrics (milliseconds)
    avg_latency: float = 0.0
    p50_latency: float = 0.0
    p95_latency: float = 0.0
    p99_latency: float = 0.0
    timeout_rate: float = 0.0

    # Data quality
    sample_size: int = 0
    sample_size_confidence: float = 0.0  # 0-1 confidence based on sample size
    last_updated: str | None = None

    # Revenue (for anchor bidder determination)
    total_revenue: float = 0.0

    def __post_init__(self):
        """Compute derived metrics."""
        self._compute_rates()
        self._compute_confidence()

    def _compute_rates(self):
        """Compute rate metrics from counts."""
        if self.request_count > 0:
            self.bid_rate = self.bid_count / self.request_count
        if self.bid_count > 0:
            self.win_rate = self.win_count / self.bid_count

    def _compute_confidence(self):
        """
        Compute confidence score based on sample size.

        Uses a logarithmic scale:
        - 100 samples = 0.3 confidence
        - 1,000 samples = 0.6 confidence
        - 10,000 samples = 0.9 confidence
        - 100,000+ samples = 1.0 confidence
        """
        import math

        if self.sample_size <= 0:
            self.sample_size_confidence = 0.0
        elif self.sample_size >= 100000:
            self.sample_size_confidence = 1.0
        else:
            # Logarithmic scale from 100 to 100,000
            self.sample_size_confidence = min(
                1.0, max(0.0, (math.log10(max(1, self.sample_size)) - 2) / 3)
            )

    @classmethod
    def empty(cls, bidder_code: str) -> "BidderMetrics":
        """Create empty metrics for a bidder with no historical data."""
        return cls(bidder_code=bidder_code)

    @classmethod
    def from_db_row(cls, row: dict) -> "BidderMetrics":
        """Create metrics from database row."""
        return cls(
            bidder_code=row.get("bidder_code", ""),
            request_count=row.get("request_count", 0),
            bid_count=row.get("bid_count", 0),
            win_count=row.get("win_count", 0),
            avg_cpm=row.get("avg_cpm", 0.0),
            max_cpm=row.get("max_cpm", 0.0),
            min_cpm=row.get("min_cpm", 0.0),
            floor_clearance_rate=row.get("floor_clearance_rate", 0.0),
            avg_latency=row.get("avg_latency", 0.0),
            p50_latency=row.get("p50_latency", 0.0),
            p95_latency=row.get("p95_latency", 0.0),
            p99_latency=row.get("p99_latency", 0.0),
            timeout_rate=row.get("timeout_rate", 0.0),
            sample_size=row.get("sample_size", row.get("request_count", 0)),
            total_revenue=row.get("total_revenue", 0.0),
            last_updated=row.get("last_updated"),
        )

    def to_dict(self) -> dict:
        """Convert to dictionary representation."""
        return {
            "bidder_code": self.bidder_code,
            "request_count": self.request_count,
            "bid_count": self.bid_count,
            "win_count": self.win_count,
            "bid_rate": self.bid_rate,
            "win_rate": self.win_rate,
            "avg_cpm": self.avg_cpm,
            "max_cpm": self.max_cpm,
            "min_cpm": self.min_cpm,
            "floor_clearance_rate": self.floor_clearance_rate,
            "avg_latency": self.avg_latency,
            "p50_latency": self.p50_latency,
            "p95_latency": self.p95_latency,
            "p99_latency": self.p99_latency,
            "timeout_rate": self.timeout_rate,
            "sample_size": self.sample_size,
            "sample_size_confidence": self.sample_size_confidence,
            "total_revenue": self.total_revenue,
            "last_updated": self.last_updated,
        }
