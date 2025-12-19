-- Migration: Add Revenue Share Columns to Publishers
-- Date: 2024-12-19
-- Description: Adds platform demand revenue share and publisher own demand fee columns
--              for tracking margins and billing on demand sources.

-- Add revenue share columns to config_publishers table
-- These columns track:
-- 1. platform_demand_rev_share: % margin taken on demand partners the platform brings in
-- 2. publisher_own_demand_fee: % fee charged when publishers use their own demand (billed monthly)

ALTER TABLE config_publishers
ADD COLUMN IF NOT EXISTS platform_demand_rev_share DECIMAL(5,2) DEFAULT 0.00;

ALTER TABLE config_publishers
ADD COLUMN IF NOT EXISTS publisher_own_demand_fee DECIMAL(5,2) DEFAULT 0.00;

-- Add comments for documentation
COMMENT ON COLUMN config_publishers.platform_demand_rev_share IS
    'Percentage margin taken from platform-sourced demand partners. Applied on top of publisher net earnings.';

COMMENT ON COLUMN config_publishers.publisher_own_demand_fee IS
    'Percentage fee charged to publisher for using their own demand sources. Billed monthly.';

-- Create index for revenue share reporting queries
CREATE INDEX IF NOT EXISTS idx_config_publishers_revenue_share
    ON config_publishers(platform_demand_rev_share, publisher_own_demand_fee)
    WHERE platform_demand_rev_share > 0 OR publisher_own_demand_fee > 0;

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
