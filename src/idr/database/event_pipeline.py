"""
Event Pipeline for auction outcome tracking.

Handles async event recording with batching and buffering
for high-throughput auction environments.
"""

import queue
import threading
import time
from collections.abc import Callable
from dataclasses import dataclass
from datetime import datetime
from enum import Enum
from typing import Any

from src.idr.database.metrics_store import MetricsStore


class EventType(Enum):
    """Types of events in the pipeline."""

    BID_REQUEST = "bid_request"
    BID_RESPONSE = "bid_response"
    WIN = "win"
    LOSS = "loss"
    TIMEOUT = "timeout"
    ERROR = "error"


@dataclass
class AuctionEvent:
    """Represents an event in the auction lifecycle."""

    event_type: EventType
    timestamp: datetime
    auction_id: str
    bidder_code: str

    # Request context
    country: str = ""
    device_type: str = ""
    media_type: str = ""
    ad_size: str = ""
    publisher_id: str = ""

    # Event data
    latency_ms: float | None = None
    bid_cpm: float | None = None
    win_cpm: float | None = None
    floor_price: float | None = None
    error_message: str | None = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for storage."""
        return {
            "event_type": self.event_type.value,
            "timestamp": self.timestamp.isoformat(),
            "auction_id": self.auction_id,
            "bidder_code": self.bidder_code,
            "country": self.country,
            "device_type": self.device_type,
            "media_type": self.media_type,
            "ad_size": self.ad_size,
            "publisher_id": self.publisher_id,
            "latency_ms": self.latency_ms,
            "bid_cpm": self.bid_cpm,
            "win_cpm": self.win_cpm,
            "floor_price": self.floor_price,
            "error_message": self.error_message,
        }


@dataclass
class PipelineStats:
    """Statistics for pipeline monitoring."""

    events_received: int = 0
    events_processed: int = 0
    events_failed: int = 0
    batches_flushed: int = 0
    last_flush_time: datetime | None = None
    avg_batch_size: float = 0.0
    queue_depth: int = 0


class EventPipeline:
    """
    Async event pipeline with batching.

    Features:
    - Non-blocking event submission
    - Configurable batch sizes and flush intervals
    - Background processing thread
    - Graceful shutdown
    """

    DEFAULT_BATCH_SIZE = 100
    DEFAULT_FLUSH_INTERVAL = 1.0  # seconds
    DEFAULT_MAX_QUEUE_SIZE = 10000

    def __init__(
        self,
        metrics_store: MetricsStore,
        batch_size: int = DEFAULT_BATCH_SIZE,
        flush_interval: float = DEFAULT_FLUSH_INTERVAL,
        max_queue_size: int = DEFAULT_MAX_QUEUE_SIZE,
        on_error: Callable[[Exception, list[AuctionEvent]], None] | None = None,
    ):
        self.metrics_store = metrics_store
        self.batch_size = batch_size
        self.flush_interval = flush_interval
        self.max_queue_size = max_queue_size
        self.on_error = on_error

        self._queue: queue.Queue[AuctionEvent] = queue.Queue(maxsize=max_queue_size)
        self._running = False
        self._thread: threading.Thread | None = None
        self._stats = PipelineStats()
        self._lock = threading.Lock()

    def start(self) -> None:
        """Start the background processing thread."""
        if self._running:
            return

        self._running = True
        self._thread = threading.Thread(target=self._process_loop, daemon=True)
        self._thread.start()

    def stop(self, timeout: float = 5.0) -> None:
        """Stop the pipeline and flush remaining events."""
        if not self._running:
            return

        self._running = False

        # Flush remaining events
        self._flush_queue()

        if self._thread:
            self._thread.join(timeout=timeout)
            self._thread = None

    def submit(self, event: AuctionEvent) -> bool:
        """
        Submit an event to the pipeline.

        Returns True if event was queued, False if queue is full.
        """
        try:
            self._queue.put_nowait(event)
            with self._lock:
                self._stats.events_received += 1
                self._stats.queue_depth = self._queue.qsize()
            return True
        except queue.Full:
            return False

    def submit_bid_response(
        self,
        auction_id: str,
        bidder_code: str,
        had_bid: bool,
        latency_ms: float,
        bid_cpm: float | None = None,
        floor_price: float | None = None,
        country: str = "",
        device_type: str = "",
        media_type: str = "",
        ad_size: str = "",
        publisher_id: str = "",
        timed_out: bool = False,
        error_message: str | None = None,
    ) -> bool:
        """Convenience method to submit a bid response event."""
        event_type = EventType.BID_RESPONSE
        if timed_out:
            event_type = EventType.TIMEOUT
        elif error_message:
            event_type = EventType.ERROR

        return self.submit(
            AuctionEvent(
                event_type=event_type,
                timestamp=datetime.now(),
                auction_id=auction_id,
                bidder_code=bidder_code,
                country=country,
                device_type=device_type,
                media_type=media_type,
                ad_size=ad_size,
                publisher_id=publisher_id,
                latency_ms=latency_ms,
                bid_cpm=bid_cpm if had_bid else None,
                floor_price=floor_price,
                error_message=error_message,
            )
        )

    def submit_win(
        self,
        auction_id: str,
        bidder_code: str,
        win_cpm: float,
        country: str = "",
        device_type: str = "",
        media_type: str = "",
        ad_size: str = "",
        publisher_id: str = "",
    ) -> bool:
        """Convenience method to submit a win event."""
        return self.submit(
            AuctionEvent(
                event_type=EventType.WIN,
                timestamp=datetime.now(),
                auction_id=auction_id,
                bidder_code=bidder_code,
                country=country,
                device_type=device_type,
                media_type=media_type,
                ad_size=ad_size,
                publisher_id=publisher_id,
                win_cpm=win_cpm,
            )
        )

    def get_stats(self) -> PipelineStats:
        """Get current pipeline statistics."""
        with self._lock:
            self._stats.queue_depth = self._queue.qsize()
            return PipelineStats(
                events_received=self._stats.events_received,
                events_processed=self._stats.events_processed,
                events_failed=self._stats.events_failed,
                batches_flushed=self._stats.batches_flushed,
                last_flush_time=self._stats.last_flush_time,
                avg_batch_size=self._stats.avg_batch_size,
                queue_depth=self._stats.queue_depth,
            )

    def _process_loop(self) -> None:
        """Background processing loop."""
        batch: list[AuctionEvent] = []
        last_flush = time.time()

        while self._running or not self._queue.empty():
            try:
                # Try to get event with timeout
                try:
                    event = self._queue.get(timeout=0.1)
                    batch.append(event)
                except queue.Empty:
                    pass

                # Check if we should flush
                should_flush = len(batch) >= self.batch_size or (
                    len(batch) > 0 and time.time() - last_flush >= self.flush_interval
                )

                if should_flush:
                    self._process_batch(batch)
                    batch = []
                    last_flush = time.time()

            except Exception as e:
                print(f"Error in pipeline loop: {e}")

        # Final flush
        if batch:
            self._process_batch(batch)

    def _flush_queue(self) -> None:
        """Flush all remaining events in queue."""
        batch: list[AuctionEvent] = []
        while not self._queue.empty():
            try:
                batch.append(self._queue.get_nowait())
            except queue.Empty:
                break

        if batch:
            self._process_batch(batch)

    def _process_batch(self, batch: list[AuctionEvent]) -> None:
        """Process a batch of events."""
        if not batch:
            return

        try:
            for event in batch:
                self._process_event(event)

            with self._lock:
                self._stats.events_processed += len(batch)
                self._stats.batches_flushed += 1
                self._stats.last_flush_time = datetime.now()
                # Rolling average of batch size
                self._stats.avg_batch_size = (
                    self._stats.avg_batch_size * 0.9 + len(batch) * 0.1
                )

        except Exception as e:
            with self._lock:
                self._stats.events_failed += len(batch)

            if self.on_error:
                self.on_error(e, batch)
            else:
                print(f"Batch processing error: {e}")

    def _process_event(self, event: AuctionEvent) -> None:
        """Process a single event."""
        # Build a minimal ClassifiedRequest for context hashing
        from src.idr.models.classified_request import ClassifiedRequest

        request = ClassifiedRequest(
            request_id=event.auction_id,
            timestamp=event.timestamp,
            country=event.country,
            device_type=event.device_type,
            ad_format=event.media_type,
            primary_size=event.ad_size,
            publisher_id=event.publisher_id,
        )

        if event.event_type == EventType.WIN:
            # Record win
            if event.win_cpm is not None:
                self.metrics_store.record_win(
                    auction_id=event.auction_id,
                    bidder_code=event.bidder_code,
                    request=request,
                    win_cpm=event.win_cpm,
                )
        else:
            # Record bid request/response
            had_bid = event.bid_cpm is not None
            timed_out = event.event_type == EventType.TIMEOUT
            had_error = event.event_type == EventType.ERROR

            self.metrics_store.record_request(
                auction_id=event.auction_id,
                bidder_code=event.bidder_code,
                request=request,
                latency_ms=event.latency_ms or 0,
                had_bid=had_bid,
                bid_cpm=event.bid_cpm,
                timed_out=timed_out,
                had_error=had_error,
                floor_price=event.floor_price,
            )


class SyncEventPipeline:
    """
    Synchronous event pipeline for testing or low-volume use.

    Processes events immediately without batching.
    """

    def __init__(self, metrics_store: MetricsStore):
        self.metrics_store = metrics_store
        self._stats = PipelineStats()

    def start(self) -> None:
        pass

    def stop(self, timeout: float = 5.0) -> None:
        pass

    def submit(self, event: AuctionEvent) -> bool:
        self._stats.events_received += 1
        try:
            self._process_event(event)
            self._stats.events_processed += 1
            return True
        except Exception:
            self._stats.events_failed += 1
            return False

    def submit_bid_response(self, **kwargs) -> bool:
        event = AuctionEvent(
            event_type=EventType.BID_RESPONSE, timestamp=datetime.now(), **kwargs
        )
        return self.submit(event)

    def submit_win(self, **kwargs) -> bool:
        event = AuctionEvent(
            event_type=EventType.WIN, timestamp=datetime.now(), **kwargs
        )
        return self.submit(event)

    def get_stats(self) -> PipelineStats:
        return self._stats

    def _process_event(self, event: AuctionEvent) -> None:
        from src.idr.models.classified_request import ClassifiedRequest

        request = ClassifiedRequest(
            request_id=event.auction_id,
            timestamp=event.timestamp,
            country=event.country,
            device_type=event.device_type,
            ad_format=event.media_type,
            primary_size=event.ad_size,
            publisher_id=event.publisher_id,
        )

        if event.event_type == EventType.WIN:
            if event.win_cpm is not None:
                self.metrics_store.record_win(
                    auction_id=event.auction_id,
                    bidder_code=event.bidder_code,
                    request=request,
                    win_cpm=event.win_cpm,
                )
        else:
            had_bid = event.bid_cpm is not None
            timed_out = event.event_type == EventType.TIMEOUT
            had_error = event.event_type == EventType.ERROR

            self.metrics_store.record_request(
                auction_id=event.auction_id,
                bidder_code=event.bidder_code,
                request=request,
                latency_ms=event.latency_ms or 0,
                had_bid=had_bid,
                bid_cpm=event.bid_cpm,
                timed_out=timed_out,
                had_error=had_error,
                floor_price=event.floor_price,
            )
