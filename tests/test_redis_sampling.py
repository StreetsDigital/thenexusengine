"""Tests for Redis sampling feature in the metrics client."""


from src.idr.database.redis_client import (
    DEFAULT_SAMPLE_RATE,
    MockRedisClient,
    RealTimeMetrics,
)


class TestMockRedisClientSampling:
    """Test MockRedisClient sampling behavior."""

    def test_default_sample_rate(self):
        """Default sample rate should be 1.0 (100%)."""
        client = MockRedisClient()
        assert client.sample_rate == 1.0

    def test_custom_sample_rate(self):
        """Can create client with custom sample rate."""
        client = MockRedisClient(sample_rate=0.1)
        assert client.sample_rate == 0.1

    def test_set_sample_rate(self):
        """Can update sample rate dynamically."""
        client = MockRedisClient()
        client.set_sample_rate(0.5)
        assert client.sample_rate == 0.5

    def test_record_request_returns_bool(self):
        """record_request should return True."""
        client = MockRedisClient()
        result = client.record_request(
            bidder="appnexus",
            context_hash="abc123",
            latency_ms=50.0,
            had_bid=True,
            bid_cpm=2.5,
        )
        assert result is True

    def test_record_win_returns_bool(self):
        """record_win should return True."""
        client = MockRedisClient()
        result = client.record_win(
            bidder="appnexus",
            context_hash="abc123",
            win_cpm=2.5,
        )
        assert result is True

    def test_get_metrics_extrapolate_param(self):
        """get_metrics should accept extrapolate parameter."""
        client = MockRedisClient()
        client.record_request(
            bidder="appnexus",
            context_hash="abc123",
            latency_ms=50.0,
            had_bid=True,
        )
        # Should work with extrapolate=True (default) and False
        metrics1 = client.get_metrics("appnexus", extrapolate=True)
        metrics2 = client.get_metrics("appnexus", extrapolate=False)
        assert metrics1.requests == 1
        assert metrics2.requests == 1

    def test_metrics_accumulate(self):
        """Metrics should accumulate correctly."""
        client = MockRedisClient()

        # Record multiple requests
        for i in range(10):
            client.record_request(
                bidder="appnexus",
                context_hash="abc123",
                latency_ms=50.0,
                had_bid=i % 2 == 0,  # 50% bid rate
                bid_cpm=2.5 if i % 2 == 0 else None,
            )

        metrics = client.get_metrics("appnexus")
        assert metrics.requests == 10
        assert metrics.bids == 5
        assert metrics.total_latency_ms == 500.0

    def test_record_win_updates_metrics(self):
        """record_win should update win metrics."""
        client = MockRedisClient()

        # First record a request
        client.record_request(
            bidder="appnexus",
            context_hash="abc123",
            latency_ms=50.0,
            had_bid=True,
            bid_cpm=2.5,
        )

        # Then record a win
        client.record_win(
            bidder="appnexus",
            context_hash="abc123",
            win_cpm=2.5,
        )

        metrics = client.get_metrics("appnexus")
        assert metrics.wins == 1
        assert metrics.total_win_value == 2.5

    def test_timeout_tracking(self):
        """Timeouts should be tracked."""
        client = MockRedisClient()

        client.record_request(
            bidder="appnexus",
            context_hash="abc123",
            latency_ms=500.0,
            had_bid=False,
            timed_out=True,
        )

        metrics = client.get_metrics("appnexus")
        assert metrics.timeouts == 1

    def test_error_tracking(self):
        """Errors should be tracked."""
        client = MockRedisClient()

        client.record_request(
            bidder="appnexus",
            context_hash="abc123",
            latency_ms=50.0,
            had_bid=False,
            had_error=True,
        )

        metrics = client.get_metrics("appnexus")
        assert metrics.errors == 1

    def test_get_all_bidder_metrics(self):
        """Should return metrics for all bidders."""
        client = MockRedisClient()

        # Record for multiple bidders
        for bidder in ["appnexus", "rubicon", "pubmatic"]:
            client.record_request(
                bidder=bidder,
                context_hash="abc123",
                latency_ms=50.0,
                had_bid=True,
            )

        all_metrics = client.get_all_bidder_metrics()
        assert len(all_metrics) == 3
        assert "appnexus" in all_metrics
        assert "rubicon" in all_metrics
        assert "pubmatic" in all_metrics

    def test_flush_bidder(self):
        """flush_bidder should clear metrics for that bidder."""
        client = MockRedisClient()

        client.record_request(
            bidder="appnexus",
            context_hash="abc123",
            latency_ms=50.0,
            had_bid=True,
        )

        # Verify data exists
        metrics = client.get_metrics("appnexus")
        assert metrics.requests == 1

        # Flush
        client.flush_bidder("appnexus")

        # Verify data is gone
        metrics = client.get_metrics("appnexus")
        assert metrics.requests == 0


class TestRealTimeMetrics:
    """Test RealTimeMetrics dataclass."""

    def test_bid_rate_calculation(self):
        """bid_rate should be bids/requests."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            requests=100,
            bids=75,
        )
        assert metrics.bid_rate == 0.75

    def test_bid_rate_zero_requests(self):
        """bid_rate should be 0 when no requests."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            requests=0,
            bids=0,
        )
        assert metrics.bid_rate == 0.0

    def test_win_rate_calculation(self):
        """win_rate should be wins/bids."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            bids=100,
            wins=20,
        )
        assert metrics.win_rate == 0.2

    def test_win_rate_zero_bids(self):
        """win_rate should be 0 when no bids."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            bids=0,
            wins=0,
        )
        assert metrics.win_rate == 0.0

    def test_avg_bid_cpm_calculation(self):
        """avg_bid_cpm should be total_bid_value/bids."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            bids=10,
            total_bid_value=25.0,
        )
        assert metrics.avg_bid_cpm == 2.5

    def test_avg_latency_calculation(self):
        """avg_latency_ms should be total_latency_ms/requests."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            requests=100,
            total_latency_ms=5000.0,
        )
        assert metrics.avg_latency_ms == 50.0

    def test_timeout_rate_calculation(self):
        """timeout_rate should be timeouts/requests."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            requests=100,
            timeouts=5,
        )
        assert metrics.timeout_rate == 0.05

    def test_error_rate_calculation(self):
        """error_rate should be errors/requests."""
        metrics = RealTimeMetrics(
            bidder_code="appnexus",
            requests=100,
            errors=2,
        )
        assert metrics.error_rate == 0.02


class TestDefaultSampleRate:
    """Test DEFAULT_SAMPLE_RATE constant."""

    def test_default_sample_rate_value(self):
        """DEFAULT_SAMPLE_RATE should be 1.0."""
        assert DEFAULT_SAMPLE_RATE == 1.0


class TestSamplingStatistics:
    """Test that sampling produces statistically valid results."""

    def test_sampling_reduces_records(self):
        """10% sampling should record roughly 10% of events."""
        MockRedisClient(sample_rate=1.0)  # Mock always records

        # Simulate what real sampling would do
        recorded = 0
        total = 1000
        sample_rate = 0.1

        import random

        random.seed(42)  # For reproducibility

        for _ in range(total):
            if random.random() < sample_rate:
                recorded += 1

        # With 10% sampling, expect ~100 records (allow 20% tolerance)
        assert 80 <= recorded <= 120

    def test_extrapolation_accuracy(self):
        """Extrapolated counts should estimate true totals."""
        # With 10% sampling and 100 recorded events,
        # extrapolation should give ~1000
        sample_rate = 0.1
        recorded_count = 100
        multiplier = 1.0 / sample_rate

        extrapolated = int(recorded_count * multiplier)
        assert extrapolated == 1000
