"""
Bidder Manager - High-level operations for managing OpenRTB bidders.

This module provides a clean API for creating, updating, and managing
custom OpenRTB bidder integrations.
"""

import os
import re
from datetime import datetime
from typing import Optional

from .models import (
    BidderConfig,
    BidderEndpoint,
    BidderCapabilities,
    BidderRateLimits,
    BidderStatus,
    RequestTransform,
    ResponseTransform,
)
from .storage import BidderStorage, get_bidder_storage


class BidderManagerError(Exception):
    """Base exception for bidder manager errors."""
    pass


class BidderNotFoundError(BidderManagerError):
    """Raised when a bidder is not found."""
    pass


class BidderAlreadyExistsError(BidderManagerError):
    """Raised when trying to create a bidder that already exists."""
    pass


class InvalidBidderConfigError(BidderManagerError):
    """Raised when bidder configuration is invalid."""
    pass


class BidderManager:
    """
    High-level manager for OpenRTB bidder configurations.

    Provides CRUD operations and validation for bidder integrations.
    """

    def __init__(self, storage: Optional[BidderStorage] = None):
        """
        Initialize the bidder manager.

        Args:
            storage: BidderStorage instance (uses global if not provided)
        """
        self.storage = storage or get_bidder_storage()

    def create_bidder(
        self,
        name: str,
        endpoint_url: str,
        media_types: list[str] = None,
        bidder_code: Optional[str] = None,
        description: str = "",
        timeout_ms: int = 200,
        protocol_version: str = "2.6",
        auth_type: Optional[str] = None,
        auth_token: Optional[str] = None,
        gvl_vendor_id: Optional[int] = None,
        priority: int = 50,
        status: BidderStatus = BidderStatus.TESTING,
        custom_headers: Optional[dict] = None,
        **kwargs,
    ) -> BidderConfig:
        """
        Create a new OpenRTB bidder.

        Args:
            name: Human-readable name for the bidder
            endpoint_url: The bidder's OpenRTB endpoint URL
            media_types: Supported media types (banner, video, audio, native)
            bidder_code: Unique identifier (auto-generated from name if not provided)
            description: Description of the bidder
            timeout_ms: Request timeout in milliseconds
            protocol_version: OpenRTB version (2.5 or 2.6)
            auth_type: Authentication type (none, bearer, header)
            auth_token: Authentication token (for bearer auth)
            gvl_vendor_id: IAB Global Vendor List ID
            priority: Bidder priority (0-100)
            status: Initial status (default: testing)
            custom_headers: Additional HTTP headers to include

        Returns:
            The created BidderConfig

        Raises:
            InvalidBidderConfigError: If configuration is invalid
            BidderAlreadyExistsError: If bidder code already exists
        """
        # Validate endpoint URL
        if not self._validate_url(endpoint_url):
            raise InvalidBidderConfigError(f"Invalid endpoint URL: {endpoint_url}")

        # Generate or validate bidder code
        if not bidder_code:
            bidder_code = self._generate_code(name)

        bidder_code = self._normalize_code(bidder_code)

        if not bidder_code:
            raise InvalidBidderConfigError("Invalid bidder code")

        # Check if already exists
        if self.storage.exists(bidder_code):
            raise BidderAlreadyExistsError(f"Bidder already exists: {bidder_code}")

        # Set default media types
        if not media_types:
            media_types = ["banner"]

        # Validate media types
        valid_media_types = {"banner", "video", "audio", "native"}
        for mt in media_types:
            if mt not in valid_media_types:
                raise InvalidBidderConfigError(f"Invalid media type: {mt}")

        # Create endpoint configuration
        endpoint = BidderEndpoint(
            url=endpoint_url,
            timeout_ms=timeout_ms,
            protocol_version=protocol_version,
            auth_type=auth_type,
            auth_token=auth_token,
            custom_headers=custom_headers or {},
        )

        # Create capabilities based on media types
        capabilities = BidderCapabilities(
            media_types=media_types,
            video_protocols=[2, 3, 5, 6] if "video" in media_types else [],
            video_mimes=["video/mp4", "video/webm"] if "video" in media_types else [],
        )

        # Create the config
        config = BidderConfig(
            bidder_code=bidder_code,
            name=name,
            description=description,
            endpoint=endpoint,
            capabilities=capabilities,
            status=status,
            gvl_vendor_id=gvl_vendor_id,
            priority=priority,
            **kwargs,
        )

        # Save to storage
        if not self.storage.save(config):
            raise BidderManagerError("Failed to save bidder configuration")

        return config

    def get_bidder(self, bidder_code: str) -> BidderConfig:
        """
        Get a bidder by code.

        Args:
            bidder_code: The bidder identifier

        Returns:
            BidderConfig

        Raises:
            BidderNotFoundError: If bidder doesn't exist
        """
        config = self.storage.get(bidder_code)
        if not config:
            raise BidderNotFoundError(f"Bidder not found: {bidder_code}")
        return config

    def update_bidder(
        self,
        bidder_code: str,
        **updates,
    ) -> BidderConfig:
        """
        Update an existing bidder.

        Args:
            bidder_code: The bidder identifier
            **updates: Fields to update

        Returns:
            Updated BidderConfig

        Raises:
            BidderNotFoundError: If bidder doesn't exist
        """
        config = self.get_bidder(bidder_code)

        # Update endpoint fields
        if any(k.startswith("endpoint_") for k in updates):
            endpoint_updates = {}
            for key, value in list(updates.items()):
                if key.startswith("endpoint_"):
                    endpoint_updates[key[9:]] = value  # Remove "endpoint_" prefix
                    del updates[key]

            if endpoint_updates:
                for k, v in endpoint_updates.items():
                    if hasattr(config.endpoint, k):
                        setattr(config.endpoint, k, v)

        # Update capabilities fields
        if any(k.startswith("capabilities_") for k in updates):
            cap_updates = {}
            for key, value in list(updates.items()):
                if key.startswith("capabilities_"):
                    cap_updates[key[13:]] = value
                    del updates[key]

            if cap_updates:
                for k, v in cap_updates.items():
                    if hasattr(config.capabilities, k):
                        setattr(config.capabilities, k, v)

        # Update top-level fields
        for key, value in updates.items():
            if hasattr(config, key):
                if key == "status" and isinstance(value, str):
                    value = BidderStatus(value)
                setattr(config, key, value)

        # Update timestamp
        config.updated_at = datetime.utcnow().isoformat()

        # Save
        if not self.storage.save(config):
            raise BidderManagerError("Failed to update bidder configuration")

        return config

    def delete_bidder(self, bidder_code: str) -> bool:
        """
        Delete a bidder.

        Args:
            bidder_code: The bidder identifier

        Returns:
            True if deleted successfully

        Raises:
            BidderNotFoundError: If bidder doesn't exist
        """
        if not self.storage.exists(bidder_code):
            raise BidderNotFoundError(f"Bidder not found: {bidder_code}")

        return self.storage.delete(bidder_code)

    def list_bidders(self, include_disabled: bool = True) -> list[BidderConfig]:
        """
        List all bidders.

        Args:
            include_disabled: Whether to include disabled bidders

        Returns:
            List of BidderConfig objects sorted by priority
        """
        if include_disabled:
            configs = list(self.storage.get_all().values())
            configs.sort(key=lambda c: c.priority, reverse=True)
            return configs
        else:
            return self.storage.get_active()

    def get_active_bidders(self) -> list[BidderConfig]:
        """Get all active bidders sorted by priority."""
        return self.storage.get_active()

    def get_bidders_for_publisher(
        self,
        publisher_id: str,
        country: Optional[str] = None,
    ) -> list[BidderConfig]:
        """
        Get bidders available for a specific publisher.

        Args:
            publisher_id: The publisher identifier
            country: Optional country code for geo filtering

        Returns:
            List of available BidderConfig objects
        """
        return self.storage.get_for_publisher(publisher_id, country)

    def enable_bidder(self, bidder_code: str) -> BidderConfig:
        """Enable a bidder (set status to active)."""
        return self.update_bidder(bidder_code, status=BidderStatus.ACTIVE)

    def disable_bidder(self, bidder_code: str) -> BidderConfig:
        """Disable a bidder."""
        return self.update_bidder(bidder_code, status=BidderStatus.DISABLED)

    def pause_bidder(self, bidder_code: str) -> BidderConfig:
        """Pause a bidder temporarily."""
        return self.update_bidder(bidder_code, status=BidderStatus.PAUSED)

    def set_testing(self, bidder_code: str) -> BidderConfig:
        """Set bidder to testing mode."""
        return self.update_bidder(bidder_code, status=BidderStatus.TESTING)

    def get_stats(self, bidder_code: str) -> dict:
        """Get real-time statistics for a bidder."""
        return self.storage.get_stats(bidder_code)

    def reset_stats(self, bidder_code: str) -> bool:
        """Reset statistics for a bidder."""
        return self.storage.reset_stats(bidder_code)

    def test_endpoint(self, bidder_code: str) -> dict:
        """
        Test a bidder's endpoint with a sample request.

        Args:
            bidder_code: The bidder identifier

        Returns:
            Test result dictionary with success status and details
        """
        import json
        import time

        try:
            import requests
        except ImportError:
            return {
                "success": False,
                "error": "requests library not available",
            }

        config = self.get_bidder(bidder_code)

        # Build a minimal test request
        test_request = {
            "id": f"test-{int(time.time())}",
            "test": 1,
            "at": 1,
            "tmax": config.endpoint.timeout_ms,
            "cur": ["USD"],
            "imp": [
                {
                    "id": "1",
                    "banner": {
                        "w": 300,
                        "h": 250,
                        "format": [{"w": 300, "h": 250}],
                    },
                    "bidfloor": 0.01,
                }
            ],
            "site": {
                "id": "test-site",
                "domain": "test.example.com",
                "page": "https://test.example.com/page",
            },
            "device": {
                "ua": "Test/1.0",
                "ip": "192.0.2.1",
            },
        }

        # Build headers
        headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
            "X-OpenRTB-Version": config.endpoint.protocol_version,
        }

        # Add authentication
        if config.endpoint.auth_type == "bearer" and config.endpoint.auth_token:
            headers["Authorization"] = f"Bearer {config.endpoint.auth_token}"
        elif config.endpoint.auth_type == "header":
            if config.endpoint.auth_header_name and config.endpoint.auth_header_value:
                headers[config.endpoint.auth_header_name] = config.endpoint.auth_header_value

        # Add custom headers
        headers.update(config.endpoint.custom_headers)

        try:
            start = time.time()
            response = requests.post(
                config.endpoint.url,
                json=test_request,
                headers=headers,
                timeout=config.endpoint.timeout_ms / 1000.0,
            )
            latency = (time.time() - start) * 1000

            result = {
                "success": response.status_code in (200, 204),
                "status_code": response.status_code,
                "latency_ms": round(latency, 2),
                "headers": dict(response.headers),
            }

            # Try to parse response
            if response.status_code == 200 and response.content:
                try:
                    body = response.json()
                    result["response"] = body
                    result["has_bids"] = bool(body.get("seatbid", []))
                    result["bid_count"] = sum(
                        len(sb.get("bid", [])) for sb in body.get("seatbid", [])
                    )
                except json.JSONDecodeError:
                    result["response_text"] = response.text[:500]
            elif response.status_code == 204:
                result["has_bids"] = False
                result["bid_count"] = 0

            return result

        except requests.Timeout:
            return {
                "success": False,
                "error": f"Request timed out after {config.endpoint.timeout_ms}ms",
            }
        except requests.RequestException as e:
            return {
                "success": False,
                "error": str(e),
            }

    def duplicate_bidder(
        self,
        source_code: str,
        new_name: str,
        new_endpoint_url: Optional[str] = None,
    ) -> BidderConfig:
        """
        Create a copy of an existing bidder with a new name.

        Args:
            source_code: The source bidder code to copy
            new_name: Name for the new bidder
            new_endpoint_url: Optional new endpoint URL

        Returns:
            The new BidderConfig
        """
        source = self.get_bidder(source_code)

        # Create new config from source
        new_code = self._generate_code(new_name)

        new_config = BidderConfig(
            bidder_code=new_code,
            name=new_name,
            description=source.description,
            endpoint=BidderEndpoint(
                url=new_endpoint_url or source.endpoint.url,
                method=source.endpoint.method,
                timeout_ms=source.endpoint.timeout_ms,
                protocol_version=source.endpoint.protocol_version,
                auth_type=source.endpoint.auth_type,
                custom_headers=source.endpoint.custom_headers.copy(),
            ),
            capabilities=BidderCapabilities(
                media_types=source.capabilities.media_types.copy(),
                currencies=source.capabilities.currencies.copy(),
                site_enabled=source.capabilities.site_enabled,
                app_enabled=source.capabilities.app_enabled,
                video_protocols=source.capabilities.video_protocols.copy(),
                video_mimes=source.capabilities.video_mimes.copy(),
            ),
            rate_limits=BidderRateLimits(
                qps_limit=source.rate_limits.qps_limit,
                daily_limit=source.rate_limits.daily_limit,
                concurrent_limit=source.rate_limits.concurrent_limit,
            ),
            request_transform=RequestTransform(
                field_mappings=source.request_transform.field_mappings.copy(),
                field_additions=source.request_transform.field_additions.copy(),
                field_removals=source.request_transform.field_removals.copy(),
                imp_ext_template=source.request_transform.imp_ext_template.copy(),
                request_ext_template=source.request_transform.request_ext_template.copy(),
            ),
            response_transform=ResponseTransform(
                bid_field_mappings=source.response_transform.bid_field_mappings.copy(),
                price_adjustment=source.response_transform.price_adjustment,
                currency_conversion=source.response_transform.currency_conversion,
            ),
            status=BidderStatus.TESTING,  # Always start in testing
            gvl_vendor_id=source.gvl_vendor_id,
            priority=source.priority,
            maintainer_email=source.maintainer_email,
        )

        if not self.storage.save(new_config):
            raise BidderManagerError("Failed to save duplicated bidder")

        return new_config

    def import_bidder(self, config_dict: dict) -> BidderConfig:
        """
        Import a bidder from a configuration dictionary.

        Args:
            config_dict: Configuration dictionary

        Returns:
            The imported BidderConfig
        """
        config = BidderConfig.from_dict(config_dict)

        # Ensure it doesn't exist
        if self.storage.exists(config.bidder_code):
            raise BidderAlreadyExistsError(f"Bidder already exists: {config.bidder_code}")

        # Reset timestamps
        config.created_at = datetime.utcnow().isoformat()
        config.updated_at = config.created_at

        # Reset stats
        config.total_requests = 0
        config.total_bids = 0
        config.total_wins = 0
        config.total_errors = 0

        if not self.storage.save(config):
            raise BidderManagerError("Failed to save imported bidder")

        return config

    def export_bidder(self, bidder_code: str) -> dict:
        """
        Export a bidder configuration as a dictionary.

        Args:
            bidder_code: The bidder identifier

        Returns:
            Configuration dictionary
        """
        config = self.get_bidder(bidder_code)
        return config.to_dict()

    @staticmethod
    def _validate_url(url: str) -> bool:
        """Validate an endpoint URL."""
        if not url:
            return False

        # Must be HTTPS in production
        if not url.startswith(("https://", "http://")):
            return False

        # Basic URL validation
        import re
        pattern = r"^https?://[a-zA-Z0-9][-a-zA-Z0-9]*(\.[a-zA-Z0-9][-a-zA-Z0-9]*)+[/a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=%-]*$"
        return bool(re.match(pattern, url))

    @staticmethod
    def _normalize_code(code: str) -> str:
        """Normalize bidder code to lowercase alphanumeric with hyphens."""
        code = code.lower().replace(" ", "-").replace("_", "-")
        code = re.sub(r"[^a-z0-9-]", "", code)
        code = re.sub(r"-+", "-", code)
        return code.strip("-")

    @staticmethod
    def _generate_code(name: str) -> str:
        """Generate a bidder code from name."""
        return BidderManager._normalize_code(name)


# Global manager instance
_bidder_manager: Optional[BidderManager] = None


def get_bidder_manager(storage: Optional[BidderStorage] = None) -> BidderManager:
    """Get the global bidder manager instance."""
    global _bidder_manager

    if _bidder_manager is None:
        _bidder_manager = BidderManager(storage)

    return _bidder_manager
