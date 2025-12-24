"""
TimescaleDB client for historical bidder performance data.

Stores aggregated metrics for analytics, ML training, and long-term trends.
Uses TimescaleDB hypertables for efficient time-series queries.

Uses connection pooling for production reliability and performance.
"""

import logging
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import datetime
from typing import Any, Generator
from urllib.parse import urlparse

try:
    import psycopg2
    import psycopg2.extras
    from psycopg2 import pool

    PSYCOPG2_AVAILABLE = True
except ImportError:
    PSYCOPG2_AVAILABLE = False

logger = logging.getLogger(__name__)


@dataclass
class BidderPerformance:
    """Historical performance record for a bidder."""

    bidder_code: str
    time_bucket: datetime

    # Context dimensions
    country: str = ""
    device_type: str = ""
    media_type: str = ""
    ad_size: str = ""
    publisher_id: str = ""

    # Metrics
    requests: int = 0
    bids: int = 0
    wins: int = 0
    timeouts: int = 0
    errors: int = 0

    total_bid_value: float = 0.0
    total_win_value: float = 0.0
    total_latency_ms: float = 0.0

    floor_clears: int = 0  # Bids above floor
    floor_total: int = 0  # Total bids with floor

    # Computed
    @property
    def bid_rate(self) -> float:
        return self.bids / self.requests if self.requests > 0 else 0.0

    @property
    def win_rate(self) -> float:
        return self.wins / self.bids if self.bids > 0 else 0.0

    @property
    def avg_bid_cpm(self) -> float:
        return self.total_bid_value / self.bids if self.bids > 0 else 0.0

    @property
    def avg_latency_ms(self) -> float:
        return self.total_latency_ms / self.requests if self.requests > 0 else 0.0

    @property
    def floor_clearance_rate(self) -> float:
        return self.floor_clears / self.floor_total if self.floor_total > 0 else 0.0


# SQL for creating tables
CREATE_TABLES_SQL = """
-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Bidder performance by context (aggregated hourly)
CREATE TABLE IF NOT EXISTS bidder_performance (
    time TIMESTAMPTZ NOT NULL,
    bidder_code TEXT NOT NULL,
    country TEXT DEFAULT '',
    device_type TEXT DEFAULT '',
    media_type TEXT DEFAULT '',
    ad_size TEXT DEFAULT '',
    publisher_id TEXT DEFAULT '',

    -- Counts
    requests BIGINT DEFAULT 0,
    bids BIGINT DEFAULT 0,
    wins BIGINT DEFAULT 0,
    timeouts BIGINT DEFAULT 0,
    errors BIGINT DEFAULT 0,

    -- Values
    total_bid_value DOUBLE PRECISION DEFAULT 0,
    total_win_value DOUBLE PRECISION DEFAULT 0,
    total_latency_ms DOUBLE PRECISION DEFAULT 0,

    -- Floor metrics
    floor_clears BIGINT DEFAULT 0,
    floor_total BIGINT DEFAULT 0
);

-- Convert to hypertable
SELECT create_hypertable('bidder_performance', 'time', if_not_exists => TRUE);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_bp_bidder_time ON bidder_performance (bidder_code, time DESC);
CREATE INDEX IF NOT EXISTS idx_bp_context ON bidder_performance (country, device_type, media_type);

-- Individual bid events (for detailed analysis)
CREATE TABLE IF NOT EXISTS bid_events (
    time TIMESTAMPTZ NOT NULL,
    auction_id TEXT NOT NULL,
    bidder_code TEXT NOT NULL,

    -- Context
    country TEXT DEFAULT '',
    device_type TEXT DEFAULT '',
    media_type TEXT DEFAULT '',
    ad_size TEXT DEFAULT '',
    publisher_id TEXT DEFAULT '',

    -- Event data
    had_bid BOOLEAN DEFAULT FALSE,
    bid_cpm DOUBLE PRECISION,
    won BOOLEAN DEFAULT FALSE,
    win_cpm DOUBLE PRECISION,
    latency_ms DOUBLE PRECISION,
    timed_out BOOLEAN DEFAULT FALSE,
    had_error BOOLEAN DEFAULT FALSE,
    floor_price DOUBLE PRECISION,
    cleared_floor BOOLEAN
);

-- Convert to hypertable
SELECT create_hypertable('bid_events', 'time', if_not_exists => TRUE);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_be_bidder_time ON bid_events (bidder_code, time DESC);
CREATE INDEX IF NOT EXISTS idx_be_auction ON bid_events (auction_id);

-- Continuous aggregate for hourly rollups
CREATE MATERIALIZED VIEW IF NOT EXISTS bidder_hourly_stats
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    bidder_code,
    country,
    device_type,
    media_type,
    COUNT(*) AS requests,
    COUNT(*) FILTER (WHERE had_bid) AS bids,
    COUNT(*) FILTER (WHERE won) AS wins,
    COUNT(*) FILTER (WHERE timed_out) AS timeouts,
    COUNT(*) FILTER (WHERE had_error) AS errors,
    SUM(bid_cpm) FILTER (WHERE had_bid) AS total_bid_value,
    SUM(win_cpm) FILTER (WHERE won) AS total_win_value,
    AVG(latency_ms) AS avg_latency_ms,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95_latency_ms,
    COUNT(*) FILTER (WHERE cleared_floor) AS floor_clears,
    COUNT(*) FILTER (WHERE floor_price IS NOT NULL) AS floor_total
FROM bid_events
GROUP BY bucket, bidder_code, country, device_type, media_type
WITH NO DATA;

-- Refresh policy (every hour)
SELECT add_continuous_aggregate_policy('bidder_hourly_stats',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- Retention policy (keep 90 days of raw events, 1 year of aggregates)
SELECT add_retention_policy('bid_events', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('bidder_performance', INTERVAL '365 days', if_not_exists => TRUE);
"""


class TimescaleClient:
    """
    TimescaleDB client for historical bidder metrics with connection pooling.

    Provides:
    - Recording bid events
    - Querying aggregated performance
    - Time-series analytics
    - Thread-safe connection pooling
    """

    # Default pool settings for production
    DEFAULT_MIN_CONNECTIONS = 2
    DEFAULT_MAX_CONNECTIONS = 10

    def __init__(
        self,
        host: str = "localhost",
        port: int = 5432,
        database: str = "idr",
        user: str = "idr",
        password: str = "",
        # Connection pool settings
        min_connections: int = DEFAULT_MIN_CONNECTIONS,
        max_connections: int = DEFAULT_MAX_CONNECTIONS,
        # URL-based connection (overrides host/port/user/password/database)
        url: str | None = None,
    ):
        if not PSYCOPG2_AVAILABLE:
            raise ImportError(
                "psycopg2 not installed. Run: pip install psycopg2-binary"
            )

        # Parse URL if provided
        if url:
            parsed = urlparse(url)
            host = parsed.hostname or "localhost"
            port = parsed.port or 5432
            user = parsed.username or "postgres"
            password = parsed.password or ""
            database = parsed.path.lstrip("/") or "idr"

        self.connection_params = {
            "host": host,
            "port": port,
            "database": database,
            "user": user,
            "password": password,
        }
        self._pool: pool.ThreadedConnectionPool | None = None
        self._min_connections = min_connections
        self._max_connections = max_connections
        self._connected = False

    def connect(self) -> bool:
        """Create connection pool and verify connectivity."""
        try:
            self._pool = pool.ThreadedConnectionPool(
                minconn=self._min_connections,
                maxconn=self._max_connections,
                **self.connection_params,
            )
            # Test the connection
            conn = self._pool.getconn()
            conn.cursor().execute("SELECT 1")
            self._pool.putconn(conn)
            self._connected = True
            logger.info(
                f"TimescaleDB pool created: {self.connection_params['host']}:{self.connection_params['port']}/{self.connection_params['database']} "
                f"(min={self._min_connections}, max={self._max_connections})"
            )
            return True
        except psycopg2.Error as e:
            logger.warning(f"Failed to connect to TimescaleDB: {e}")
            self._connected = False
            return False

    def close(self) -> None:
        """Close all connections in the pool."""
        if self._pool:
            self._pool.closeall()
            self._pool = None
            self._connected = False
            logger.info("TimescaleDB connection pool closed")

    @property
    def is_connected(self) -> bool:
        return self._connected and self._pool is not None

    @contextmanager
    def get_connection(self) -> Generator[Any, None, None]:
        """Get a connection from the pool with automatic return."""
        if not self._pool:
            raise RuntimeError("Connection pool not initialized. Call connect() first.")

        conn = self._pool.getconn()
        try:
            yield conn
        finally:
            self._pool.putconn(conn)

    def get_pool_stats(self) -> dict[str, Any]:
        """Get connection pool statistics for monitoring."""
        if not self._pool:
            return {"pool_available": False}

        # Note: psycopg2 pool doesn't expose detailed stats, so we provide config
        return {
            "pool_available": True,
            "min_connections": self._min_connections,
            "max_connections": self._max_connections,
        }

    def initialize_schema(self) -> bool:
        """Create tables and hypertables if they don't exist."""
        if not self.is_connected:
            return False

        try:
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    # Split and execute statements separately
                    statements = [
                        s.strip() for s in CREATE_TABLES_SQL.split(";") if s.strip()
                    ]
                    for stmt in statements:
                        try:
                            cur.execute(stmt)
                        except psycopg2.Error as e:
                            # Ignore "already exists" errors
                            if "already exists" not in str(e):
                                logger.warning(f"Schema init warning: {e}")
                conn.commit()
            return True
        except psycopg2.Error as e:
            logger.error(f"Failed to initialize schema: {e}")
            return False

    # =========================================================================
    # Recording Events
    # =========================================================================

    def record_bid_event(
        self,
        auction_id: str,
        bidder_code: str,
        country: str = "",
        device_type: str = "",
        media_type: str = "",
        ad_size: str = "",
        publisher_id: str = "",
        had_bid: bool = False,
        bid_cpm: float | None = None,
        won: bool = False,
        win_cpm: float | None = None,
        latency_ms: float | None = None,
        timed_out: bool = False,
        had_error: bool = False,
        floor_price: float | None = None,
    ) -> bool:
        """Record a single bid event."""
        if not self.is_connected:
            return False

        cleared_floor = None
        if floor_price is not None and bid_cpm is not None:
            cleared_floor = bid_cpm >= floor_price

        try:
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO bid_events (
                            time, auction_id, bidder_code, country, device_type,
                            media_type, ad_size, publisher_id, had_bid, bid_cpm,
                            won, win_cpm, latency_ms, timed_out, had_error,
                            floor_price, cleared_floor
                        ) VALUES (
                            NOW(), %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
                        )
                    """,
                        (
                            auction_id,
                            bidder_code,
                            country,
                            device_type,
                            media_type,
                            ad_size,
                            publisher_id,
                            had_bid,
                            bid_cpm,
                            won,
                            win_cpm,
                            latency_ms,
                            timed_out,
                            had_error,
                            floor_price,
                            cleared_floor,
                        ),
                    )
                conn.commit()
            return True
        except psycopg2.Error as e:
            logger.error(f"Failed to record event: {e}")
            return False

    def record_batch_events(self, events: list[dict]) -> int:
        """Record multiple bid events efficiently."""
        if not self.is_connected or not events:
            return 0

        try:
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    psycopg2.extras.execute_batch(
                        cur,
                        """
                        INSERT INTO bid_events (
                            time, auction_id, bidder_code, country, device_type,
                            media_type, ad_size, publisher_id, had_bid, bid_cpm,
                            won, win_cpm, latency_ms, timed_out, had_error,
                            floor_price, cleared_floor
                        ) VALUES (
                            NOW(), %(auction_id)s, %(bidder_code)s, %(country)s, %(device_type)s,
                            %(media_type)s, %(ad_size)s, %(publisher_id)s, %(had_bid)s, %(bid_cpm)s,
                            %(won)s, %(win_cpm)s, %(latency_ms)s, %(timed_out)s, %(had_error)s,
                            %(floor_price)s, %(cleared_floor)s
                        )
                    """,
                        events,
                    )
                conn.commit()
            return len(events)
        except psycopg2.Error as e:
            logger.error(f"Failed to record batch: {e}")
            return 0

    # =========================================================================
    # Querying Performance
    # =========================================================================

    def get_bidder_performance(
        self,
        bidder_code: str,
        hours: int = 24,
        country: str | None = None,
        device_type: str | None = None,
        media_type: str | None = None,
    ) -> BidderPerformance | None:
        """Get aggregated performance for a bidder."""
        if not self.is_connected:
            return None

        where_clauses = ["bidder_code = %s", "time > NOW() - INTERVAL '%s hours'"]
        params: list[Any] = [bidder_code, hours]

        if country:
            where_clauses.append("country = %s")
            params.append(country)
        if device_type:
            where_clauses.append("device_type = %s")
            params.append(device_type)
        if media_type:
            where_clauses.append("media_type = %s")
            params.append(media_type)

        query = f"""
            SELECT
                COUNT(*) as requests,
                COUNT(*) FILTER (WHERE had_bid) as bids,
                COUNT(*) FILTER (WHERE won) as wins,
                COUNT(*) FILTER (WHERE timed_out) as timeouts,
                COUNT(*) FILTER (WHERE had_error) as errors,
                COALESCE(SUM(bid_cpm) FILTER (WHERE had_bid), 0) as total_bid_value,
                COALESCE(SUM(win_cpm) FILTER (WHERE won), 0) as total_win_value,
                COALESCE(SUM(latency_ms), 0) as total_latency_ms,
                COUNT(*) FILTER (WHERE cleared_floor) as floor_clears,
                COUNT(*) FILTER (WHERE floor_price IS NOT NULL) as floor_total
            FROM bid_events
            WHERE {" AND ".join(where_clauses)}
        """

        try:
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    cur.execute(query, params)
                    row = cur.fetchone()

                    if row and row[0] > 0:
                        return BidderPerformance(
                            bidder_code=bidder_code,
                            time_bucket=datetime.now(),
                            country=country or "",
                            device_type=device_type or "",
                            media_type=media_type or "",
                            requests=row[0],
                            bids=row[1],
                            wins=row[2],
                            timeouts=row[3],
                            errors=row[4],
                            total_bid_value=float(row[5]),
                            total_win_value=float(row[6]),
                            total_latency_ms=float(row[7]),
                            floor_clears=row[8],
                            floor_total=row[9],
                        )
                    return None
        except psycopg2.Error as e:
            logger.error(f"Query failed: {e}")
            return None

    def get_p95_latency(self, bidder_code: str, hours: int = 1) -> float:
        """Get P95 latency for a bidder."""
        if not self.is_connected:
            return 0.0

        try:
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        SELECT PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms)
                        FROM bid_events
                        WHERE bidder_code = %s
                        AND time > NOW() - INTERVAL '%s hours'
                        AND latency_ms IS NOT NULL
                    """,
                        (bidder_code, hours),
                    )
                    row = cur.fetchone()
                    return float(row[0]) if row and row[0] else 0.0
        except psycopg2.Error:
            return 0.0

    def get_all_bidder_stats(self, hours: int = 24) -> list[BidderPerformance]:
        """Get stats for all bidders."""
        if not self.is_connected:
            return []

        try:
            with self.get_connection() as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        SELECT
                            bidder_code,
                            COUNT(*) as requests,
                            COUNT(*) FILTER (WHERE had_bid) as bids,
                            COUNT(*) FILTER (WHERE won) as wins,
                            COUNT(*) FILTER (WHERE timed_out) as timeouts,
                            COUNT(*) FILTER (WHERE had_error) as errors,
                            COALESCE(SUM(bid_cpm) FILTER (WHERE had_bid), 0) as total_bid_value,
                            COALESCE(SUM(win_cpm) FILTER (WHERE won), 0) as total_win_value,
                            COALESCE(SUM(latency_ms), 0) as total_latency_ms,
                            COUNT(*) FILTER (WHERE cleared_floor) as floor_clears,
                            COUNT(*) FILTER (WHERE floor_price IS NOT NULL) as floor_total
                        FROM bid_events
                        WHERE time > NOW() - INTERVAL '%s hours'
                        GROUP BY bidder_code
                        ORDER BY requests DESC
                    """,
                        (hours,),
                    )

                    results = []
                    for row in cur.fetchall():
                        results.append(
                            BidderPerformance(
                                bidder_code=row[0],
                                time_bucket=datetime.now(),
                                requests=row[1],
                                bids=row[2],
                                wins=row[3],
                                timeouts=row[4],
                                errors=row[5],
                                total_bid_value=float(row[6]),
                                total_win_value=float(row[7]),
                                total_latency_ms=float(row[8]),
                                floor_clears=row[9],
                                floor_total=row[10],
                            )
                        )
                    return results
        except psycopg2.Error as e:
            logger.error(f"Query failed: {e}")
            return []


class MockTimescaleClient:
    """In-memory mock for testing without TimescaleDB."""

    def __init__(self):
        self._events: list[dict] = []
        self._connected = True

    def connect(self) -> bool:
        return True

    def close(self) -> None:
        pass

    @property
    def is_connected(self) -> bool:
        return self._connected

    def initialize_schema(self) -> bool:
        return True

    def record_bid_event(self, auction_id: str, bidder_code: str, **kwargs) -> bool:
        self._events.append(
            {"auction_id": auction_id, "bidder_code": bidder_code, **kwargs}
        )
        return True

    def record_batch_events(self, events: list[dict]) -> int:
        self._events.extend(events)
        return len(events)

    def get_bidder_performance(
        self, bidder_code: str, **kwargs
    ) -> BidderPerformance | None:
        events = [e for e in self._events if e.get("bidder_code") == bidder_code]
        if not events:
            return None

        return BidderPerformance(
            bidder_code=bidder_code,
            time_bucket=datetime.now(),
            requests=len(events),
            bids=sum(1 for e in events if e.get("had_bid")),
            wins=sum(1 for e in events if e.get("won")),
            total_bid_value=sum(
                e.get("bid_cpm", 0) or 0 for e in events if e.get("had_bid")
            ),
            total_latency_ms=sum(e.get("latency_ms", 0) or 0 for e in events),
        )

    def get_p95_latency(self, bidder_code: str, hours: int = 1) -> float:
        latencies = [
            e.get("latency_ms", 0)
            for e in self._events
            if e.get("bidder_code") == bidder_code and e.get("latency_ms")
        ]
        if not latencies:
            return 0.0
        latencies.sort()
        return latencies[int(len(latencies) * 0.95)]

    def get_all_bidder_stats(self, hours: int = 24) -> list[BidderPerformance]:
        bidders = {e.get("bidder_code") for e in self._events}
        return [self.get_bidder_performance(b) for b in bidders if b]
