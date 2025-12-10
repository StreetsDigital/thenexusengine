-- TimescaleDB Schema for The Nexus Engine IDR
-- Initialize database schema on first run

-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- ===========================================
-- Auction Events Table (Hypertable)
-- ===========================================
CREATE TABLE IF NOT EXISTS auction_events (
    timestamp TIMESTAMPTZ NOT NULL,
    auction_id TEXT NOT NULL,
    publisher_id TEXT NOT NULL,

    -- Request attributes
    country TEXT,
    device_type TEXT,
    ad_format TEXT,
    ad_size TEXT,
    floor_price DECIMAL(10,4),

    -- Bidder response
    bidder_code TEXT NOT NULL,
    did_bid BOOLEAN NOT NULL DEFAULT FALSE,
    bid_cpm DECIMAL(10,4),
    response_time_ms INTEGER,
    did_win BOOLEAN NOT NULL DEFAULT FALSE,

    -- Additional context
    timed_out BOOLEAN DEFAULT FALSE,
    had_error BOOLEAN DEFAULT FALSE,
    error_message TEXT,

    -- IDs present
    user_ids JSONB
);

-- Convert to hypertable (partitioned by time)
SELECT create_hypertable('auction_events', 'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- ===========================================
-- Indexes for fast lookups
-- ===========================================
CREATE INDEX IF NOT EXISTS idx_auction_events_bidder
    ON auction_events (bidder_code, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_auction_events_publisher
    ON auction_events (publisher_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_auction_events_lookup
    ON auction_events (bidder_code, publisher_id, country, device_type, ad_format, timestamp DESC);

-- ===========================================
-- Continuous Aggregate: Hourly Performance
-- ===========================================
CREATE MATERIALIZED VIEW IF NOT EXISTS bidder_performance_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', timestamp) AS hour,
    bidder_code,
    publisher_id,
    country,
    device_type,
    ad_format,
    ad_size,

    -- Counts
    COUNT(*) AS request_count,
    SUM(CASE WHEN did_bid THEN 1 ELSE 0 END) AS bid_count,
    SUM(CASE WHEN did_win THEN 1 ELSE 0 END) AS win_count,
    SUM(CASE WHEN timed_out THEN 1 ELSE 0 END) AS timeout_count,
    SUM(CASE WHEN had_error THEN 1 ELSE 0 END) AS error_count,

    -- Metrics
    AVG(bid_cpm) FILTER (WHERE did_bid) AS avg_cpm,
    AVG(response_time_ms) AS avg_latency_ms,

    -- For percentile calculation (store as array, compute in app)
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time_ms)
        FILTER (WHERE response_time_ms IS NOT NULL) AS p95_latency_ms,

    -- Floor clearance
    SUM(CASE WHEN did_bid AND bid_cpm >= floor_price THEN 1 ELSE 0 END)::FLOAT /
        NULLIF(SUM(CASE WHEN did_bid THEN 1 ELSE 0 END), 0) AS floor_clearance_rate

FROM auction_events
GROUP BY 1, 2, 3, 4, 5, 6, 7
WITH NO DATA;

-- Refresh policy: update hourly view every 5 minutes
SELECT add_continuous_aggregate_policy('bidder_performance_hourly',
    start_offset => INTERVAL '2 hours',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE
);

-- ===========================================
-- Continuous Aggregate: Daily Performance
-- ===========================================
CREATE MATERIALIZED VIEW IF NOT EXISTS bidder_performance_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', timestamp) AS day,
    bidder_code,
    publisher_id,
    country,
    device_type,
    ad_format,

    COUNT(*) AS request_count,
    SUM(CASE WHEN did_bid THEN 1 ELSE 0 END) AS bid_count,
    SUM(CASE WHEN did_win THEN 1 ELSE 0 END) AS win_count,
    AVG(bid_cpm) FILTER (WHERE did_bid) AS avg_cpm,
    SUM(bid_cpm) FILTER (WHERE did_win) AS total_revenue

FROM auction_events
GROUP BY 1, 2, 3, 4, 5, 6
WITH NO DATA;

-- Refresh policy: update daily view every hour
SELECT add_continuous_aggregate_policy('bidder_performance_daily',
    start_offset => INTERVAL '2 days',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

-- ===========================================
-- Data Retention Policies
-- ===========================================

-- Keep raw events for 7 days
SELECT add_retention_policy('auction_events',
    INTERVAL '7 days',
    if_not_exists => TRUE
);

-- Keep hourly aggregates for 30 days
SELECT add_retention_policy('bidder_performance_hourly',
    INTERVAL '30 days',
    if_not_exists => TRUE
);

-- Keep daily aggregates for 1 year
SELECT add_retention_policy('bidder_performance_daily',
    INTERVAL '365 days',
    if_not_exists => TRUE
);

-- ===========================================
-- Helper function: Get bidder metrics
-- ===========================================
CREATE OR REPLACE FUNCTION get_bidder_metrics(
    p_bidder_code TEXT,
    p_publisher_id TEXT DEFAULT NULL,
    p_country TEXT DEFAULT NULL,
    p_device_type TEXT DEFAULT NULL,
    p_ad_format TEXT DEFAULT NULL,
    p_hours INTEGER DEFAULT 24
)
RETURNS TABLE (
    request_count BIGINT,
    bid_count BIGINT,
    win_count BIGINT,
    bid_rate FLOAT,
    win_rate FLOAT,
    avg_cpm NUMERIC,
    p95_latency_ms FLOAT,
    floor_clearance_rate FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        SUM(h.request_count)::BIGINT,
        SUM(h.bid_count)::BIGINT,
        SUM(h.win_count)::BIGINT,
        SUM(h.bid_count)::FLOAT / NULLIF(SUM(h.request_count), 0),
        SUM(h.win_count)::FLOAT / NULLIF(SUM(h.request_count), 0),
        AVG(h.avg_cpm),
        AVG(h.p95_latency_ms),
        AVG(h.floor_clearance_rate)
    FROM bidder_performance_hourly h
    WHERE h.bidder_code = p_bidder_code
      AND h.hour >= NOW() - (p_hours || ' hours')::INTERVAL
      AND (p_publisher_id IS NULL OR h.publisher_id = p_publisher_id)
      AND (p_country IS NULL OR h.country = p_country)
      AND (p_device_type IS NULL OR h.device_type = p_device_type)
      AND (p_ad_format IS NULL OR h.ad_format = p_ad_format);
END;
$$ LANGUAGE plpgsql;

-- ===========================================
-- Grant permissions
-- ===========================================
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO postgres;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO postgres;
