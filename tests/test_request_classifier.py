"""Tests for the Request Classifier."""

from datetime import datetime

import pytest

from src.idr.classifier import RequestClassifier
from src.idr.models.classified_request import AdFormat, AdPosition, DeviceType


class TestRequestClassifier:
    """Test suite for RequestClassifier."""

    @pytest.fixture
    def classifier(self):
        """Create a classifier instance."""
        return RequestClassifier()

    @pytest.fixture
    def sample_banner_request(self):
        """Sample OpenRTB banner request."""
        return {
            "id": "auction-123",
            "imp": [
                {
                    "id": "imp-1",
                    "banner": {
                        "format": [{"w": 300, "h": 250}, {"w": 320, "h": 50}],
                        "pos": 1,
                    },
                    "bidfloor": 0.50,
                    "bidfloorcur": "USD",
                }
            ],
            "site": {
                "id": "site-456",
                "domain": "example.com",
                "page": "https://example.com/article/news-story",
                "cat": ["IAB12", "IAB12-2"],
                "publisher": {"id": "pub-789"},
            },
            "device": {
                "ua": "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
                "devicetype": 4,
                "geo": {"country": "GB", "region": "LND"},
                "connectiontype": 2,
            },
            "user": {
                "id": "user-abc",
                "ext": {
                    "consent": "BOEFEAyOEFEAyAHABDENAI4AAAB9vABAASA",
                    "eids": [
                        {"source": "uidapi.com", "uids": [{"id": "uid2-value-123"}]},
                        {"source": "id5-sync.com", "uids": [{"id": "id5-value-456"}]},
                    ],
                },
            },
            "regs": {"ext": {"gdpr": 1}},
        }

    @pytest.fixture
    def sample_video_request(self):
        """Sample OpenRTB video request."""
        return {
            "id": "video-auction-123",
            "imp": [
                {
                    "id": "imp-video-1",
                    "video": {
                        "mimes": ["video/mp4"],
                        "w": 640,
                        "h": 480,
                        "minduration": 5,
                        "maxduration": 30,
                    },
                    "bidfloor": 5.00,
                }
            ],
            "site": {
                "id": "video-site",
                "domain": "videos.example.com",
                "publisher": {"id": "video-pub"},
            },
            "device": {
                "ua": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0",
                "devicetype": 2,
                "geo": {"country": "US", "metro": "501"},
            },
            "user": {},
        }

    def test_classify_banner_format(self, classifier, sample_banner_request):
        """Test banner format classification."""
        result = classifier.classify(sample_banner_request)

        assert result.ad_format == AdFormat.BANNER
        assert result.ad_sizes == ["300x250", "320x50"]
        assert result.primary_ad_size == "300x250"

    def test_classify_video_format(self, classifier, sample_video_request):
        """Test video format classification."""
        result = classifier.classify(sample_video_request)

        assert result.ad_format == AdFormat.VIDEO
        assert result.ad_sizes == ["640x480"]

    def test_classify_position(self, classifier, sample_banner_request):
        """Test position classification."""
        result = classifier.classify(sample_banner_request)

        assert result.position == AdPosition.ATF

    def test_classify_device_mobile(self, classifier, sample_banner_request):
        """Test mobile device classification."""
        result = classifier.classify(sample_banner_request)

        assert result.device_type == DeviceType.MOBILE
        assert result.is_mobile is True
        assert result.os == "ios"
        assert result.browser == "safari"

    def test_classify_device_desktop(self, classifier, sample_video_request):
        """Test desktop device classification."""
        result = classifier.classify(sample_video_request)

        assert result.device_type == DeviceType.DESKTOP
        assert result.is_mobile is False
        assert result.browser == "chrome"

    def test_classify_geo(self, classifier, sample_banner_request):
        """Test geo classification."""
        result = classifier.classify(sample_banner_request)

        assert result.country == "GB"
        assert result.region == "LND"
        assert result.dma is None

    def test_classify_geo_with_dma(self, classifier, sample_video_request):
        """Test geo classification with DMA."""
        result = classifier.classify(sample_video_request)

        assert result.country == "US"
        assert result.dma == "501"

    def test_classify_publisher(self, classifier, sample_banner_request):
        """Test publisher classification."""
        result = classifier.classify(sample_banner_request)

        assert result.publisher_id == "pub-789"
        assert result.site_id == "site-456"
        assert result.domain == "example.com"

    def test_classify_page_type(self, classifier, sample_banner_request):
        """Test page type inference."""
        result = classifier.classify(sample_banner_request)

        assert result.page_type == "article"

    def test_classify_categories(self, classifier, sample_banner_request):
        """Test category extraction."""
        result = classifier.classify(sample_banner_request)

        assert "IAB12" in result.categories
        assert "IAB12-2" in result.categories

    def test_classify_user_ids(self, classifier, sample_banner_request):
        """Test user ID extraction from EIDs."""
        result = classifier.classify(sample_banner_request)

        assert result.has_user_ids is True
        assert "uid2" in result.user_ids
        assert result.user_ids["uid2"] == "uid2-value-123"
        assert "id5" in result.user_ids
        assert result.user_ids["id5"] == "id5-value-456"

    def test_classify_consent(self, classifier, sample_banner_request):
        """Test consent classification."""
        result = classifier.classify(sample_banner_request)

        assert result.has_consent is True
        assert result.consent_string is not None

    def test_classify_floor_price(self, classifier, sample_banner_request):
        """Test floor price extraction."""
        result = classifier.classify(sample_banner_request)

        assert result.floor_price == 0.50
        assert result.floor_currency == "USD"

    def test_classify_connection_type(self, classifier, sample_banner_request):
        """Test connection type classification."""
        result = classifier.classify(sample_banner_request)

        assert result.connection_type == "wifi"

    def test_classify_timestamp(self, classifier, sample_banner_request):
        """Test timestamp is set."""
        result = classifier.classify(sample_banner_request)

        assert result.timestamp is not None
        assert isinstance(result.timestamp, datetime)
        assert 0 <= result.hour_of_day <= 23
        assert 0 <= result.day_of_week <= 6

    def test_classify_empty_request(self, classifier):
        """Test handling of empty request."""
        result = classifier.classify({})

        assert result.impression_id == ""
        assert result.ad_format == AdFormat.BANNER
        assert result.device_type == DeviceType.DESKTOP
        assert result.country == "UNKNOWN"  # Country codes are uppercased

    def test_classify_minimal_request(self, classifier):
        """Test handling of minimal request."""
        minimal_request = {
            "imp": [{"id": "minimal-imp", "banner": {"w": 728, "h": 90}}]
        }

        result = classifier.classify(minimal_request)

        assert result.impression_id == "minimal-imp"
        assert result.ad_sizes == ["728x90"]
        assert result.ad_format == AdFormat.BANNER

    def test_to_dict(self, classifier, sample_banner_request):
        """Test conversion to dictionary."""
        result = classifier.classify(sample_banner_request)
        result_dict = result.to_dict()

        assert isinstance(result_dict, dict)
        assert result_dict["ad_format"] == "banner"
        assert result_dict["device_type"] == "mobile"
        assert result_dict["country"] == "GB"


class TestUserAgentParsing:
    """Test user agent parsing functionality."""

    @pytest.fixture
    def classifier(self):
        return RequestClassifier()

    def test_chrome_desktop(self, classifier):
        """Test Chrome on desktop."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "device": {
                "ua": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36"
            },
        }
        result = classifier.classify(request)

        assert result.browser == "chrome"
        assert result.os == "windows"
        assert result.device_type == DeviceType.DESKTOP

    def test_safari_iphone(self, classifier):
        """Test Safari on iPhone."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 320, "h": 50}}],
            "device": {
                "ua": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile Safari/604.1"
            },
        }
        result = classifier.classify(request)

        assert result.browser == "safari"
        assert result.os == "ios"
        assert result.device_type == DeviceType.MOBILE

    def test_firefox_android(self, classifier):
        """Test Firefox on Android."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 320, "h": 50}}],
            "device": {
                "ua": "Mozilla/5.0 (Android 13; Mobile; rv:120.0) Gecko/120.0 Firefox/120.0"
            },
        }
        result = classifier.classify(request)

        assert result.browser == "firefox"
        assert result.os == "android"
        assert result.device_type == DeviceType.MOBILE


class TestPublisherIdAutoPopulate:
    """Test auto-population of publisher IDs."""

    @pytest.fixture
    def classifier(self):
        return RequestClassifier()

    def test_auto_populate_when_missing(self, classifier):
        """Test that publisher ID is auto-generated when not provided."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com"},
        }
        result = classifier.classify(request)

        # Should auto-generate with pub_ prefix
        assert result.publisher_id.startswith("pub_")
        # Should have alphanumeric random part
        assert len(result.publisher_id) == 16  # "pub_" + 12 chars

    def test_auto_populate_empty_publisher_object(self, classifier):
        """Test auto-generate when publisher object exists but has no ID."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com", "publisher": {}},
        }
        result = classifier.classify(request)

        assert result.publisher_id.startswith("pub_")

    def test_uses_provided_publisher_id(self, classifier):
        """Test that provided publisher ID is used when available."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com", "publisher": {"id": "my-publisher-123"}},
        }
        result = classifier.classify(request)

        assert result.publisher_id == "my-publisher-123"

    def test_uses_site_id_as_fallback(self, classifier):
        """Test that site ID is used if publisher ID is missing."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"id": "site-id-456", "domain": "example.com", "publisher": {}},
        }
        result = classifier.classify(request)

        assert result.publisher_id == "site-id-456"

    def test_auto_populate_uniqueness(self, classifier):
        """Test that auto-generated IDs are unique across requests."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com"},
        }

        ids = [classifier.classify(request).publisher_id for _ in range(50)]
        assert len(set(ids)) == 50  # All should be unique


class TestSiteIdAutoPopulate:
    """Test auto-population of site IDs."""

    @pytest.fixture
    def classifier(self):
        return RequestClassifier()

    def test_auto_populate_when_missing(self, classifier):
        """Test that site ID is auto-generated when not provided."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com"},
        }
        result = classifier.classify(request)

        # Should auto-generate with site_ prefix
        assert result.site_id.startswith("site_")
        # Should have alphanumeric random part
        assert len(result.site_id) == 17  # "site_" + 12 chars

    def test_uses_provided_site_id(self, classifier):
        """Test that provided site ID is used when available."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"id": "my-site-123", "domain": "example.com"},
        }
        result = classifier.classify(request)

        assert result.site_id == "my-site-123"

    def test_auto_populate_uniqueness(self, classifier):
        """Test that auto-generated site IDs are unique across requests."""
        request = {
            "imp": [{"id": "1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com"},
        }

        ids = [classifier.classify(request).site_id for _ in range(50)]
        assert len(set(ids)) == 50  # All should be unique


class TestAdUnitIdAutoPopulate:
    """Test auto-population of ad unit IDs (from tagid field)."""

    @pytest.fixture
    def classifier(self):
        return RequestClassifier()

    def test_auto_populate_when_missing(self, classifier):
        """Test that ad unit ID is auto-generated when tagid not provided."""
        request = {
            "imp": [{"id": "imp-1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com"},
        }
        result = classifier.classify(request)

        # Should auto-generate with unit_ prefix
        assert result.ad_unit_id.startswith("unit_")
        # Should have alphanumeric random part
        assert len(result.ad_unit_id) == 17  # "unit_" + 12 chars

    def test_uses_provided_tagid(self, classifier):
        """Test that provided tagid is used for ad_unit_id."""
        request = {
            "imp": [
                {
                    "id": "imp-1",
                    "tagid": "header-banner-300x250",
                    "banner": {"w": 300, "h": 250},
                }
            ],
            "site": {"domain": "example.com"},
        }
        result = classifier.classify(request)

        assert result.ad_unit_id == "header-banner-300x250"

    def test_auto_populate_empty_tagid(self, classifier):
        """Test auto-generate when tagid is empty."""
        request = {
            "imp": [{"id": "imp-1", "tagid": "", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com"},
        }
        result = classifier.classify(request)

        assert result.ad_unit_id.startswith("unit_")

    def test_auto_populate_uniqueness(self, classifier):
        """Test that auto-generated ad unit IDs are unique."""
        request = {
            "imp": [{"id": "imp-1", "banner": {"w": 300, "h": 250}}],
            "site": {"domain": "example.com"},
        }

        ids = [classifier.classify(request).ad_unit_id for _ in range(50)]
        assert len(set(ids)) == 50  # All should be unique

    def test_impression_id_separate_from_ad_unit_id(self, classifier):
        """Test that impression_id and ad_unit_id are separate fields."""
        request = {
            "imp": [
                {
                    "id": "request-123",
                    "tagid": "sidebar-ad",
                    "banner": {"w": 300, "h": 250},
                }
            ],
            "site": {"domain": "example.com"},
        }
        result = classifier.classify(request)

        assert result.impression_id == "request-123"
        assert result.ad_unit_id == "sidebar-ad"
