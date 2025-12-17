"""
Tests for OpenRTB Bidder Management System

These tests verify the functionality of the dynamic bidder system including:
- Bidder model creation and serialization
- Storage operations (Redis mock)
- Manager CRUD operations
- Bidder configuration validation
"""

import json
import pytest
from datetime import datetime
from unittest.mock import MagicMock, patch

# Import the bidder modules
from src.idr.bidders.models import (
    BidderConfig,
    BidderEndpoint,
    BidderCapabilities,
    BidderRateLimits,
    BidderStatus,
    RequestTransform,
    ResponseTransform,
)
from src.idr.bidders.storage import BidderStorage
from src.idr.bidders.manager import (
    BidderManager,
    BidderNotFoundError,
    BidderAlreadyExistsError,
    InvalidBidderConfigError,
)


class TestBidderModels:
    """Test bidder configuration models."""

    def test_bidder_endpoint_creation(self):
        """Test creating a bidder endpoint configuration."""
        endpoint = BidderEndpoint(
            url="https://dsp.example.com/bid",
            timeout_ms=200,
            protocol_version="2.6",
            auth_type="bearer",
            auth_token="secret123",
        )

        assert endpoint.url == "https://dsp.example.com/bid"
        assert endpoint.timeout_ms == 200
        assert endpoint.protocol_version == "2.6"
        assert endpoint.auth_type == "bearer"
        assert endpoint.auth_token == "secret123"
        assert endpoint.method == "POST"

    def test_bidder_endpoint_to_dict(self):
        """Test endpoint serialization."""
        endpoint = BidderEndpoint(
            url="https://dsp.example.com/bid",
            custom_headers={"X-Custom": "value"},
        )

        data = endpoint.to_dict()
        assert data["url"] == "https://dsp.example.com/bid"
        assert data["custom_headers"] == {"X-Custom": "value"}

    def test_bidder_endpoint_from_dict(self):
        """Test endpoint deserialization."""
        data = {
            "url": "https://dsp.example.com/bid",
            "timeout_ms": 300,
            "auth_type": "header",
            "auth_header_name": "X-API-Key",
            "auth_header_value": "key123",
        }

        endpoint = BidderEndpoint.from_dict(data)
        assert endpoint.url == "https://dsp.example.com/bid"
        assert endpoint.timeout_ms == 300
        assert endpoint.auth_header_name == "X-API-Key"

    def test_bidder_capabilities_defaults(self):
        """Test default capabilities."""
        caps = BidderCapabilities()

        assert caps.media_types == ["banner"]
        assert caps.currencies == ["USD"]
        assert caps.site_enabled is True
        assert caps.app_enabled is True
        assert caps.supports_gdpr is True
        assert caps.supports_ccpa is True

    def test_bidder_capabilities_custom(self):
        """Test custom capabilities."""
        caps = BidderCapabilities(
            media_types=["banner", "video"],
            currencies=["USD", "EUR"],
            video_protocols=[2, 3, 5],
            video_mimes=["video/mp4"],
            supports_ctv=True,
        )

        assert "video" in caps.media_types
        assert "EUR" in caps.currencies
        assert caps.supports_ctv is True

    def test_bidder_config_creation(self):
        """Test creating a complete bidder configuration."""
        config = BidderConfig(
            bidder_code="my-dsp",
            name="My DSP",
            description="A test DSP",
            endpoint=BidderEndpoint(url="https://dsp.example.com/bid"),
            status=BidderStatus.ACTIVE,
            priority=75,
        )

        assert config.bidder_code == "my-dsp"
        assert config.name == "My DSP"
        assert config.status == BidderStatus.ACTIVE
        assert config.priority == 75
        assert config.is_enabled is True

    def test_bidder_config_code_normalization(self):
        """Test that bidder codes are normalized."""
        config = BidderConfig(
            bidder_code="My DSP_Partner",
            name="Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
        )

        assert config.bidder_code == "my-dsp-partner"

    def test_bidder_config_serialization(self):
        """Test full config serialization round-trip."""
        original = BidderConfig(
            bidder_code="test-bidder",
            name="Test Bidder",
            description="For testing",
            endpoint=BidderEndpoint(
                url="https://test.example.com/bid",
                timeout_ms=250,
            ),
            capabilities=BidderCapabilities(
                media_types=["banner", "video"],
            ),
            status=BidderStatus.TESTING,
            priority=60,
            gvl_vendor_id=123,
            allowed_countries=["US", "CA"],
        )

        # Serialize to dict
        data = original.to_dict()

        # Deserialize back
        restored = BidderConfig.from_dict(data)

        assert restored.bidder_code == original.bidder_code
        assert restored.name == original.name
        assert restored.endpoint.url == original.endpoint.url
        assert restored.endpoint.timeout_ms == original.endpoint.timeout_ms
        assert restored.capabilities.media_types == original.capabilities.media_types
        assert restored.status == original.status
        assert restored.gvl_vendor_id == original.gvl_vendor_id
        assert restored.allowed_countries == original.allowed_countries

    def test_bidder_config_json(self):
        """Test JSON serialization."""
        config = BidderConfig(
            bidder_code="json-test",
            name="JSON Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
        )

        json_str = config.to_json()
        restored = BidderConfig.from_json(json_str)

        assert restored.bidder_code == "json-test"

    def test_bidder_config_computed_properties(self):
        """Test computed properties like bid_rate and win_rate."""
        config = BidderConfig(
            bidder_code="stats-test",
            name="Stats Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
            total_requests=1000,
            total_bids=250,
            total_wins=50,
            total_errors=10,
        )

        assert config.bid_rate == 0.25
        assert config.win_rate == 0.2
        assert config.error_rate == 0.01


class TestBidderStorage:
    """Test bidder storage with mocked Redis."""

    @pytest.fixture
    def mock_redis(self):
        """Create a mock Redis client."""
        redis = MagicMock()
        redis.ping.return_value = True
        redis.hset.return_value = True
        redis.hget.return_value = None
        redis.hdel.return_value = 1
        redis.hexists.return_value = False
        redis.hkeys.return_value = []
        redis.hgetall.return_value = {}
        redis.smembers.return_value = set()
        redis.sadd.return_value = 1
        redis.srem.return_value = 1
        redis.zadd.return_value = 1
        redis.zrem.return_value = 1
        redis.pipeline.return_value.__enter__ = MagicMock(return_value=redis)
        redis.pipeline.return_value.__exit__ = MagicMock(return_value=False)
        redis.pipeline.return_value.execute.return_value = [True]
        return redis

    @pytest.fixture
    def storage(self, mock_redis):
        """Create storage with mocked Redis."""
        # Create storage without connecting to real Redis
        storage = BidderStorage.__new__(BidderStorage)
        storage._redis_url = "redis://localhost:6379"
        storage._redis = mock_redis
        return storage

    def test_storage_connection(self, storage, mock_redis):
        """Test storage Redis connection."""
        assert storage.is_connected is True
        mock_redis.ping.assert_called()

    def test_storage_save(self, storage, mock_redis):
        """Test saving a bidder configuration."""
        config = BidderConfig(
            bidder_code="save-test",
            name="Save Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
        )

        # Mock pipeline
        pipeline = MagicMock()
        mock_redis.pipeline.return_value = pipeline
        pipeline.execute.return_value = [True, True, True]

        result = storage.save(config)
        assert result is True

    def test_storage_get(self, storage, mock_redis):
        """Test retrieving a bidder configuration."""
        config = BidderConfig(
            bidder_code="get-test",
            name="Get Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
        )

        mock_redis.hget.return_value = config.to_json()

        result = storage.get("get-test")
        assert result is not None
        assert result.bidder_code == "get-test"

    def test_storage_get_not_found(self, storage, mock_redis):
        """Test getting non-existent bidder."""
        mock_redis.hget.return_value = None

        result = storage.get("nonexistent")
        assert result is None

    def test_storage_exists(self, storage, mock_redis):
        """Test checking bidder existence."""
        mock_redis.hexists.return_value = True
        assert storage.exists("test-bidder") is True

        mock_redis.hexists.return_value = False
        assert storage.exists("nonexistent") is False


class TestBidderManager:
    """Test bidder manager operations."""

    @pytest.fixture
    def mock_storage(self):
        """Create a mock storage."""
        storage = MagicMock(spec=BidderStorage)
        storage.is_connected = True
        storage.exists.return_value = False
        storage.save.return_value = True
        storage.delete.return_value = True
        storage.get_stats.return_value = {}
        storage.get_all.return_value = {}
        storage.get_active.return_value = []
        storage.list_codes.return_value = []
        return storage

    @pytest.fixture
    def manager(self, mock_storage):
        """Create manager with mocked storage."""
        return BidderManager(storage=mock_storage)

    def test_create_bidder_basic(self, manager, mock_storage):
        """Test creating a basic bidder."""
        bidder = manager.create_bidder(
            name="Test DSP",
            endpoint_url="https://dsp.example.com/bid",
        )

        assert bidder.bidder_code == "test-dsp"
        assert bidder.name == "Test DSP"
        assert bidder.endpoint.url == "https://dsp.example.com/bid"
        assert "banner" in bidder.capabilities.media_types
        mock_storage.save.assert_called_once()

    def test_create_bidder_with_options(self, manager, mock_storage):
        """Test creating a bidder with full options."""
        bidder = manager.create_bidder(
            name="Full DSP",
            endpoint_url="https://dsp.example.com/bid",
            media_types=["banner", "video"],
            timeout_ms=300,
            protocol_version="2.5",
            auth_type="bearer",
            auth_token="secret",
            gvl_vendor_id=456,
            priority=80,
        )

        assert bidder.bidder_code == "full-dsp"
        assert bidder.endpoint.timeout_ms == 300
        assert bidder.endpoint.protocol_version == "2.5"
        assert bidder.endpoint.auth_type == "bearer"
        assert bidder.gvl_vendor_id == 456
        assert bidder.priority == 80
        assert "video" in bidder.capabilities.media_types

    def test_create_bidder_custom_code(self, manager, mock_storage):
        """Test creating a bidder with custom code."""
        bidder = manager.create_bidder(
            name="Custom DSP",
            endpoint_url="https://dsp.example.com/bid",
            bidder_code="my-custom-code",
        )

        assert bidder.bidder_code == "my-custom-code"

    def test_create_bidder_invalid_url(self, manager):
        """Test that invalid URLs are rejected."""
        with pytest.raises(InvalidBidderConfigError):
            manager.create_bidder(
                name="Bad DSP",
                endpoint_url="not-a-valid-url",
            )

    def test_create_bidder_invalid_media_type(self, manager):
        """Test that invalid media types are rejected."""
        with pytest.raises(InvalidBidderConfigError):
            manager.create_bidder(
                name="Bad DSP",
                endpoint_url="https://dsp.example.com/bid",
                media_types=["banner", "invalid_type"],
            )

    def test_create_bidder_already_exists(self, manager, mock_storage):
        """Test that duplicate bidders are rejected."""
        mock_storage.exists.return_value = True

        with pytest.raises(BidderAlreadyExistsError):
            manager.create_bidder(
                name="Duplicate DSP",
                endpoint_url="https://dsp.example.com/bid",
            )

    def test_get_bidder(self, manager, mock_storage):
        """Test getting a bidder."""
        config = BidderConfig(
            bidder_code="get-test",
            name="Get Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
        )
        mock_storage.get.return_value = config

        result = manager.get_bidder("get-test")
        assert result.bidder_code == "get-test"

    def test_get_bidder_not_found(self, manager, mock_storage):
        """Test getting non-existent bidder."""
        mock_storage.get.return_value = None

        with pytest.raises(BidderNotFoundError):
            manager.get_bidder("nonexistent")

    def test_update_bidder(self, manager, mock_storage):
        """Test updating a bidder."""
        existing = BidderConfig(
            bidder_code="update-test",
            name="Update Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
            priority=50,
        )
        mock_storage.get.return_value = existing

        updated = manager.update_bidder(
            "update-test",
            name="Updated Name",
            priority=75,
        )

        assert updated.name == "Updated Name"
        assert updated.priority == 75
        mock_storage.save.assert_called_once()

    def test_delete_bidder(self, manager, mock_storage):
        """Test deleting a bidder."""
        mock_storage.exists.return_value = True

        result = manager.delete_bidder("delete-test")
        assert result is True
        mock_storage.delete.assert_called_once_with("delete-test")

    def test_delete_bidder_not_found(self, manager, mock_storage):
        """Test deleting non-existent bidder."""
        mock_storage.exists.return_value = False

        with pytest.raises(BidderNotFoundError):
            manager.delete_bidder("nonexistent")

    def test_enable_bidder(self, manager, mock_storage):
        """Test enabling a bidder."""
        existing = BidderConfig(
            bidder_code="enable-test",
            name="Enable Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
            status=BidderStatus.DISABLED,
        )
        mock_storage.get.return_value = existing

        updated = manager.enable_bidder("enable-test")
        assert updated.status == BidderStatus.ACTIVE

    def test_disable_bidder(self, manager, mock_storage):
        """Test disabling a bidder."""
        existing = BidderConfig(
            bidder_code="disable-test",
            name="Disable Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
            status=BidderStatus.ACTIVE,
        )
        mock_storage.get.return_value = existing

        updated = manager.disable_bidder("disable-test")
        assert updated.status == BidderStatus.DISABLED

    def test_list_bidders(self, manager, mock_storage):
        """Test listing all bidders."""
        configs = {
            "bidder1": BidderConfig(
                bidder_code="bidder1",
                name="Bidder 1",
                endpoint=BidderEndpoint(url="https://b1.example.com/bid"),
                priority=80,
            ),
            "bidder2": BidderConfig(
                bidder_code="bidder2",
                name="Bidder 2",
                endpoint=BidderEndpoint(url="https://b2.example.com/bid"),
                priority=60,
            ),
        }
        mock_storage.get_all.return_value = configs

        bidders = manager.list_bidders()
        assert len(bidders) == 2
        # Should be sorted by priority (descending)
        assert bidders[0].bidder_code == "bidder1"

    def test_import_bidder(self, manager, mock_storage):
        """Test importing a bidder from dict."""
        config_dict = {
            "bidder_code": "imported",
            "name": "Imported DSP",
            "endpoint": {
                "url": "https://imported.example.com/bid",
                "timeout_ms": 200,
            },
            "capabilities": {
                "media_types": ["banner", "video"],
            },
            "status": "testing",
        }

        mock_storage.exists.return_value = False

        bidder = manager.import_bidder(config_dict)
        assert bidder.bidder_code == "imported"
        assert bidder.name == "Imported DSP"
        mock_storage.save.assert_called_once()

    def test_export_bidder(self, manager, mock_storage):
        """Test exporting a bidder to dict."""
        config = BidderConfig(
            bidder_code="export-test",
            name="Export Test",
            endpoint=BidderEndpoint(url="https://example.com/bid"),
        )
        mock_storage.get.return_value = config

        exported = manager.export_bidder("export-test")
        assert exported["bidder_code"] == "export-test"
        assert exported["name"] == "Export Test"
        assert "endpoint" in exported

    def test_duplicate_bidder(self, manager, mock_storage):
        """Test duplicating a bidder."""
        source = BidderConfig(
            bidder_code="source",
            name="Source DSP",
            endpoint=BidderEndpoint(
                url="https://source.example.com/bid",
                timeout_ms=250,
            ),
            capabilities=BidderCapabilities(media_types=["banner", "video"]),
            priority=70,
        )
        mock_storage.get.return_value = source
        mock_storage.exists.side_effect = [True, False]  # source exists, copy doesn't

        copy = manager.duplicate_bidder(
            source_code="source",
            new_name="Source DSP Copy",
        )

        assert copy.bidder_code == "source-dsp-copy"
        assert copy.name == "Source DSP Copy"
        assert copy.endpoint.timeout_ms == 250
        assert copy.priority == 70
        assert copy.status == BidderStatus.TESTING


class TestRequestTransform:
    """Test request transformation rules."""

    def test_request_transform_defaults(self):
        """Test default transformation."""
        transform = RequestTransform()

        assert transform.field_mappings == {}
        assert transform.field_additions == {}
        assert transform.field_removals == []

    def test_request_transform_with_templates(self):
        """Test transformation with templates."""
        transform = RequestTransform(
            imp_ext_template={"bidder": {"account_id": "12345"}},
            request_ext_template={"custom": "value"},
            seat_id="my-seat",
        )

        data = transform.to_dict()
        assert data["imp_ext_template"]["bidder"]["account_id"] == "12345"
        assert data["seat_id"] == "my-seat"


class TestResponseTransform:
    """Test response transformation rules."""

    def test_response_transform_defaults(self):
        """Test default response transformation."""
        transform = ResponseTransform()

        assert transform.price_adjustment == 1.0
        assert transform.currency_conversion is True

    def test_response_transform_with_fee(self):
        """Test response transformation with fee adjustment."""
        transform = ResponseTransform(
            price_adjustment=0.95,  # 5% fee
        )

        assert transform.price_adjustment == 0.95


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
