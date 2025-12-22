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

-- Keep raw events for 3 days (hourly aggregates make raw data redundant after 2 days)
SELECT add_retention_policy('auction_events',
    INTERVAL '3 days',
    if_not_exists => TRUE
);

-- Keep hourly aggregates for 14 days (daily aggregates sufficient after 7 days)
SELECT add_retention_policy('bidder_performance_hourly',
    INTERVAL '14 days',
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
-- Configuration Tables (Hierarchical Config)
-- ===========================================

-- Global configuration
CREATE TABLE IF NOT EXISTS config_global (
    id INTEGER PRIMARY KEY DEFAULT 1,
    config JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT single_global_config CHECK (id = 1)
);

-- Publisher configurations
CREATE TABLE IF NOT EXISTS config_publishers (
    publisher_id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN DEFAULT TRUE,
    contact_email TEXT DEFAULT '',
    contact_name TEXT DEFAULT '',
    bidders JSONB DEFAULT '{}',
    api_key TEXT DEFAULT '',
    api_key_enabled BOOLEAN DEFAULT TRUE,
    features JSONB DEFAULT '{}',
    -- Revenue Share Configuration
    -- Platform demand rev share: % taken from platform-sourced demand partners (applied on top of publisher net)
    platform_demand_rev_share DECIMAL(5,2) DEFAULT 0.00,
    -- Publisher own demand fee: % charged to publisher for using their own demand sources (billed monthly)
    publisher_own_demand_fee DECIMAL(5,2) DEFAULT 0.00,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Site configurations
CREATE TABLE IF NOT EXISTS config_sites (
    site_id TEXT NOT NULL,
    publisher_id TEXT NOT NULL REFERENCES config_publishers(publisher_id) ON DELETE CASCADE,
    domain TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN DEFAULT TRUE,
    features JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (publisher_id, site_id)
);

-- Ad unit configurations
CREATE TABLE IF NOT EXISTS config_ad_units (
    unit_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    publisher_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN DEFAULT TRUE,
    sizes JSONB DEFAULT '[]',
    media_type TEXT DEFAULT 'banner',
    position TEXT DEFAULT 'unknown',
    floor_price DECIMAL(10,4),
    floor_currency TEXT DEFAULT 'USD',
    video JSONB,
    features JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (publisher_id, site_id, unit_id),
    FOREIGN KEY (publisher_id, site_id) REFERENCES config_sites(publisher_id, site_id) ON DELETE CASCADE
);

-- Indexes for config tables
CREATE INDEX IF NOT EXISTS idx_config_publishers_enabled ON config_publishers(enabled);
CREATE INDEX IF NOT EXISTS idx_config_sites_publisher ON config_sites(publisher_id);
CREATE INDEX IF NOT EXISTS idx_config_sites_domain ON config_sites(domain);
CREATE INDEX IF NOT EXISTS idx_config_ad_units_site ON config_ad_units(publisher_id, site_id);

-- Function to update timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for auto-updating timestamps
DROP TRIGGER IF EXISTS update_config_global_updated_at ON config_global;
CREATE TRIGGER update_config_global_updated_at
    BEFORE UPDATE ON config_global
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_config_publishers_updated_at ON config_publishers;
CREATE TRIGGER update_config_publishers_updated_at
    BEFORE UPDATE ON config_publishers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_config_sites_updated_at ON config_sites;
CREATE TRIGGER update_config_sites_updated_at
    BEFORE UPDATE ON config_sites
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_config_ad_units_updated_at ON config_ad_units;
CREATE TRIGGER update_config_ad_units_updated_at
    BEFORE UPDATE ON config_ad_units
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ===========================================
-- Revenue Share Tracking Views and Functions
-- ===========================================

-- View: Publisher Revenue with Applied Revenue Share
-- Calculates net revenue, platform margins, and fees per publisher
CREATE OR REPLACE VIEW publisher_revenue_summary AS
SELECT
    p.publisher_id,
    p.name AS publisher_name,
    p.platform_demand_rev_share,
    p.publisher_own_demand_fee,
    COALESCE(perf.total_revenue, 0) AS gross_revenue,
    COALESCE(perf.win_count, 0) AS total_wins,
    COALESCE(perf.request_count, 0) AS total_requests,
    -- Platform demand margin (what we keep)
    COALESCE(perf.total_revenue, 0) * p.platform_demand_rev_share / 100 AS platform_margin,
    -- Publisher net (what they get from platform demand)
    COALESCE(perf.total_revenue, 0) * (100 - p.platform_demand_rev_share) / 100 AS publisher_net_from_platform,
    -- Own demand fee (what we bill them for their own demand)
    COALESCE(perf.total_revenue, 0) * p.publisher_own_demand_fee / 100 AS own_demand_fee_billable
FROM config_publishers p
LEFT JOIN (
    SELECT
        publisher_id,
        SUM(total_revenue) AS total_revenue,
        SUM(win_count) AS win_count,
        SUM(request_count) AS request_count
    FROM bidder_performance_daily
    WHERE day >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY publisher_id
) perf ON p.publisher_id = perf.publisher_id;

-- Function: Get revenue breakdown for a specific publisher
CREATE OR REPLACE FUNCTION get_publisher_revenue_breakdown(
    p_publisher_id TEXT,
    p_start_date DATE DEFAULT CURRENT_DATE - INTERVAL '30 days',
    p_end_date DATE DEFAULT CURRENT_DATE
)
RETURNS TABLE (
    period DATE,
    gross_revenue DECIMAL,
    platform_margin DECIMAL,
    publisher_net DECIMAL,
    own_demand_fee DECIMAL,
    win_count BIGINT,
    avg_cpm DECIMAL
) AS $$
DECLARE
    v_platform_share DECIMAL;
    v_own_demand_fee DECIMAL;
BEGIN
    -- Get revenue share config for publisher
    SELECT
        COALESCE(platform_demand_rev_share, 0),
        COALESCE(publisher_own_demand_fee, 0)
    INTO v_platform_share, v_own_demand_fee
    FROM config_publishers
    WHERE publisher_id = p_publisher_id;

    RETURN QUERY
    SELECT
        perf.day AS period,
        COALESCE(SUM(perf.total_revenue), 0) AS gross_revenue,
        COALESCE(SUM(perf.total_revenue), 0) * v_platform_share / 100 AS platform_margin,
        COALESCE(SUM(perf.total_revenue), 0) * (100 - v_platform_share) / 100 AS publisher_net,
        COALESCE(SUM(perf.total_revenue), 0) * v_own_demand_fee / 100 AS own_demand_fee,
        SUM(perf.win_count)::BIGINT AS win_count,
        AVG(perf.avg_cpm) AS avg_cpm
    FROM bidder_performance_daily perf
    WHERE perf.publisher_id = p_publisher_id
      AND perf.day >= p_start_date
      AND perf.day <= p_end_date
    GROUP BY perf.day
    ORDER BY perf.day DESC;
END;
$$ LANGUAGE plpgsql;

-- Function: Calculate monthly billing for a publisher
CREATE OR REPLACE FUNCTION get_publisher_monthly_billing(
    p_publisher_id TEXT,
    p_year INTEGER DEFAULT EXTRACT(YEAR FROM CURRENT_DATE)::INTEGER,
    p_month INTEGER DEFAULT EXTRACT(MONTH FROM CURRENT_DATE)::INTEGER
)
RETURNS TABLE (
    billing_month TEXT,
    gross_revenue DECIMAL,
    platform_margin DECIMAL,
    publisher_net DECIMAL,
    own_demand_fee_billed DECIMAL,
    total_impressions BIGINT
) AS $$
DECLARE
    v_platform_share DECIMAL;
    v_own_demand_fee DECIMAL;
    v_month_start DATE;
    v_month_end DATE;
BEGIN
    -- Calculate month boundaries
    v_month_start := make_date(p_year, p_month, 1);
    v_month_end := (v_month_start + INTERVAL '1 month' - INTERVAL '1 day')::DATE;

    -- Get revenue share config
    SELECT
        COALESCE(platform_demand_rev_share, 0),
        COALESCE(publisher_own_demand_fee, 0)
    INTO v_platform_share, v_own_demand_fee
    FROM config_publishers
    WHERE publisher_id = p_publisher_id;

    RETURN QUERY
    SELECT
        TO_CHAR(v_month_start, 'YYYY-MM') AS billing_month,
        COALESCE(SUM(perf.total_revenue), 0) AS gross_revenue,
        COALESCE(SUM(perf.total_revenue), 0) * v_platform_share / 100 AS platform_margin,
        COALESCE(SUM(perf.total_revenue), 0) * (100 - v_platform_share) / 100 AS publisher_net,
        COALESCE(SUM(perf.total_revenue), 0) * v_own_demand_fee / 100 AS own_demand_fee_billed,
        SUM(perf.win_count)::BIGINT AS total_impressions
    FROM bidder_performance_daily perf
    WHERE perf.publisher_id = p_publisher_id
      AND perf.day >= v_month_start
      AND perf.day <= v_month_end;
END;
$$ LANGUAGE plpgsql;

-- View: Platform-wide revenue summary with margins
CREATE OR REPLACE VIEW platform_revenue_summary AS
SELECT
    DATE_TRUNC('month', perf.day)::DATE AS month,
    COUNT(DISTINCT perf.publisher_id) AS active_publishers,
    SUM(perf.total_revenue) AS total_gross_revenue,
    SUM(perf.total_revenue * COALESCE(p.platform_demand_rev_share, 0) / 100) AS total_platform_margins,
    SUM(perf.total_revenue * COALESCE(p.publisher_own_demand_fee, 0) / 100) AS total_own_demand_fees,
    SUM(perf.win_count) AS total_impressions,
    AVG(perf.avg_cpm) AS avg_cpm_across_publishers
FROM bidder_performance_daily perf
LEFT JOIN config_publishers p ON perf.publisher_id = p.publisher_id
GROUP BY DATE_TRUNC('month', perf.day)
ORDER BY month DESC;

-- ===========================================
-- Grant permissions
-- ===========================================
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO postgres;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO postgres;
