-- Migration: Add Revenue Share Columns to Publishers
-- Date: 2024-12-19
-- Description: Adds platform demand revenue share and publisher own demand fee columns
--              for tracking margins and billing on demand sources.
--
-- NOTE: Views and functions that use these columns are defined in init.sql.
--       This migration only adds the columns for existing databases.

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
