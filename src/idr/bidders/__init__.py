"""
OpenRTB Bidder Management Module

This module provides functionality to create, configure, and manage
custom OpenRTB bidders that can be dynamically added to the auction system.

Components:
- models.py: Data models for bidder configuration
- storage.py: Redis-based storage for bidder configurations
- manager.py: High-level bidder management operations

Usage:
    from src.idr.bidders import BidderManager, get_bidder_manager

    manager = get_bidder_manager()

    # Create a new bidder
    bidder = manager.create_bidder(
        name="My DSP",
        endpoint="https://dsp.example.com/bid",
        supported_media_types=["banner", "video"],
    )

    # List all bidders
    bidders = manager.list_bidders()

    # Get a specific bidder
    bidder = manager.get_bidder("my-dsp")
"""

from .models import (
    BidderConfig,
    BidderEndpoint,
    BidderCapabilities,
    BidderRateLimits,
    BidderStatus,
    RequestTransform,
    ResponseTransform,
)
from .storage import BidderStorage
from .manager import BidderManager, get_bidder_manager

__all__ = [
    "BidderConfig",
    "BidderEndpoint",
    "BidderCapabilities",
    "BidderRateLimits",
    "BidderStatus",
    "RequestTransform",
    "ResponseTransform",
    "BidderStorage",
    "BidderManager",
    "get_bidder_manager",
]
