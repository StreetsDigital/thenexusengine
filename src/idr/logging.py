"""
Structured logging configuration for the IDR service.

Provides consistent JSON logging with request correlation IDs,
structured fields, and configurable log levels.
"""

import logging
import os
import sys
import time
import uuid
from contextvars import ContextVar
from functools import wraps
from typing import Any, Callable

import structlog

# Context variable for request ID
request_id_var: ContextVar[str] = ContextVar("request_id", default="")


def get_request_id() -> str:
    """Get the current request ID from context."""
    return request_id_var.get()


def set_request_id(request_id: str) -> None:
    """Set the request ID in context."""
    request_id_var.set(request_id)


def generate_request_id() -> str:
    """Generate a new unique request ID."""
    return f"{int(time.time())}-{uuid.uuid4().hex[:8]}"


def add_request_id(
    logger: structlog.typing.WrappedLogger,
    method_name: str,
    event_dict: dict[str, Any],
) -> dict[str, Any]:
    """Processor to add request ID to log entries."""
    request_id = get_request_id()
    if request_id:
        event_dict["request_id"] = request_id
    return event_dict


def add_service_info(
    logger: structlog.typing.WrappedLogger,
    method_name: str,
    event_dict: dict[str, Any],
) -> dict[str, Any]:
    """Processor to add service info to log entries."""
    event_dict["service"] = "idr"
    return event_dict


def configure_logging(
    level: str = "INFO",
    format: str = "json",
    show_timestamps: bool = True,
) -> None:
    """
    Configure structured logging for the application.

    Args:
        level: Log level (DEBUG, INFO, WARNING, ERROR)
        format: Output format ('json' or 'console')
        show_timestamps: Whether to include timestamps
    """
    # Get configuration from environment
    level = os.getenv("LOG_LEVEL", level).upper()
    format = os.getenv("LOG_FORMAT", format).lower()

    # Configure standard library logging
    logging.basicConfig(
        format="%(message)s",
        stream=sys.stdout,
        level=getattr(logging, level, logging.INFO),
    )

    # Build processor chain
    processors: list[structlog.typing.Processor] = [
        structlog.contextvars.merge_contextvars,
        add_request_id,
        add_service_info,
        structlog.stdlib.add_log_level,
        structlog.stdlib.PositionalArgumentsFormatter(),
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
        structlog.processors.UnicodeDecoder(),
    ]

    if show_timestamps:
        processors.insert(0, structlog.processors.TimeStamper(fmt="iso"))

    # Choose renderer based on format
    if format == "console":
        processors.append(
            structlog.dev.ConsoleRenderer(colors=sys.stdout.isatty())
        )
    else:
        processors.append(structlog.processors.JSONRenderer())

    # Configure structlog
    structlog.configure(
        processors=processors,
        wrapper_class=structlog.stdlib.BoundLogger,
        context_class=dict,
        logger_factory=structlog.stdlib.LoggerFactory(),
        cache_logger_on_first_use=True,
    )


def get_logger(name: str = __name__) -> structlog.stdlib.BoundLogger:
    """
    Get a structured logger instance.

    Args:
        name: Logger name (typically __name__)

    Returns:
        Configured structured logger
    """
    return structlog.get_logger(name)


# Pre-configured loggers for different components
def auction_logger() -> structlog.stdlib.BoundLogger:
    """Get logger for auction-related events."""
    return get_logger("idr.auction")


def bidder_logger(bidder_code: str) -> structlog.stdlib.BoundLogger:
    """Get logger for bidder-specific events."""
    return get_logger("idr.bidder").bind(bidder=bidder_code)


def privacy_logger() -> structlog.stdlib.BoundLogger:
    """Get logger for privacy filter events."""
    return get_logger("idr.privacy")


def database_logger() -> structlog.stdlib.BoundLogger:
    """Get logger for database operations."""
    return get_logger("idr.database")


def http_logger() -> structlog.stdlib.BoundLogger:
    """Get logger for HTTP operations."""
    return get_logger("idr.http")


class LogContext:
    """Context manager for request-scoped logging."""

    def __init__(self, request_id: str | None = None, **initial_context: Any):
        """
        Initialize log context.

        Args:
            request_id: Optional request ID (generated if not provided)
            **initial_context: Additional context to bind
        """
        self.request_id = request_id or generate_request_id()
        self.initial_context = initial_context
        self.token = None

    def __enter__(self) -> "LogContext":
        """Enter context and set request ID."""
        self.token = request_id_var.set(self.request_id)
        if self.initial_context:
            structlog.contextvars.bind_contextvars(**self.initial_context)
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        """Exit context and clear request ID."""
        request_id_var.reset(self.token)
        structlog.contextvars.clear_contextvars()


def log_execution_time(logger: structlog.stdlib.BoundLogger | None = None):
    """
    Decorator to log function execution time.

    Args:
        logger: Optional logger instance (uses default if not provided)
    """
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs):
            _logger = logger or get_logger(func.__module__)
            start = time.perf_counter()
            try:
                result = func(*args, **kwargs)
                duration_ms = (time.perf_counter() - start) * 1000
                _logger.debug(
                    "Function executed",
                    function=func.__name__,
                    duration_ms=round(duration_ms, 2),
                )
                return result
            except Exception as e:
                duration_ms = (time.perf_counter() - start) * 1000
                _logger.error(
                    "Function failed",
                    function=func.__name__,
                    duration_ms=round(duration_ms, 2),
                    error=str(e),
                    exc_info=True,
                )
                raise
        return wrapper
    return decorator


# Initialize with defaults on module load
configure_logging()
