"""Tests for the hierarchical configuration system."""

import pytest

from src.idr.config.feature_config import (
    BidderSettings,
    ConfigLevel,
    FeatureConfig,
    FeatureFlags,
    FloorSettings,
    IDRSettings,
    PrivacySettings,
    RateLimitSettings,
    ResolvedConfig,
    get_default_global_config,
)
from src.idr.config.config_resolver import (
    AdUnitConfig,
    ConfigResolver,
    PublisherConfigV2,
    SiteConfig,
    get_config_resolver,
    merge_configs,
    resolve_config,
    to_resolved_config,
)


class TestIDRSettings:
    """Test IDRSettings dataclass."""

    def test_default_values(self):
        """Test that IDRSettings has all None defaults."""
        settings = IDRSettings()
        assert settings.enabled is None
        assert settings.max_bidders is None
        assert settings.bypass_enabled is None

    def test_from_dict(self):
        """Test creating IDRSettings from dictionary."""
        data = {
            "enabled": True,
            "max_bidders": 10,
            "min_score_threshold": 30.0,
            "exploration_rate": 0.2,
        }
        settings = IDRSettings.from_dict(data)

        assert settings.enabled is True
        assert settings.max_bidders == 10
        assert settings.min_score_threshold == 30.0
        assert settings.exploration_rate == 0.2
        # Unspecified should be None
        assert settings.bypass_enabled is None

    def test_to_dict_excludes_none(self):
        """Test that to_dict excludes None values."""
        settings = IDRSettings(enabled=True, max_bidders=10)
        result = settings.to_dict()

        assert "enabled" in result
        assert "max_bidders" in result
        assert result["enabled"] is True
        assert result["max_bidders"] == 10
        # None values should not be in output
        assert "bypass_enabled" not in result or result.get("bypass_enabled") is None


class TestFeatureConfig:
    """Test FeatureConfig dataclass."""

    def test_default_values(self):
        """Test default FeatureConfig."""
        config = FeatureConfig()
        assert config.config_id == ""
        assert config.config_level == ConfigLevel.GLOBAL
        assert config.enabled is True

    def test_from_dict_round_trip(self):
        """Test that from_dict and to_dict are reversible."""
        original = FeatureConfig(
            config_id="test-config",
            config_level=ConfigLevel.PUBLISHER,
            name="Test Config",
            idr=IDRSettings(enabled=True, max_bidders=12),
            privacy=PrivacySettings(gdpr_applies=True),
        )

        data = original.to_dict()
        restored = FeatureConfig.from_dict(data)

        assert restored.config_id == original.config_id
        assert restored.name == original.name
        assert restored.idr.max_bidders == original.idr.max_bidders
        assert restored.privacy.gdpr_applies == original.privacy.gdpr_applies

    def test_get_default_global_config(self):
        """Test default global config has all settings."""
        config = get_default_global_config()

        assert config.config_id == "global"
        assert config.config_level == ConfigLevel.GLOBAL
        assert config.idr.enabled is True
        assert config.idr.max_bidders == 15
        assert config.privacy.gdpr_applies is True


class TestMergeConfigs:
    """Test configuration merging."""

    def test_child_overrides_parent(self):
        """Test that child values override parent values."""
        parent = FeatureConfig(
            config_id="parent",
            idr=IDRSettings(max_bidders=15, min_score_threshold=25.0),
        )
        child = FeatureConfig(
            config_id="child",
            idr=IDRSettings(max_bidders=10),  # Override only max_bidders
        )

        merged = merge_configs(child, parent)

        assert merged.config_id == "child"
        assert merged.idr.max_bidders == 10  # From child
        assert merged.idr.min_score_threshold == 25.0  # From parent

    def test_none_inherits_from_parent(self):
        """Test that None values inherit from parent."""
        parent = FeatureConfig(
            config_id="parent",
            idr=IDRSettings(
                enabled=True,
                max_bidders=15,
                exploration_rate=0.1,
            ),
        )
        child = FeatureConfig(
            config_id="child",
            idr=IDRSettings(
                max_bidders=10,
                # enabled and exploration_rate not specified
            ),
        )

        merged = merge_configs(child, parent)

        assert merged.idr.max_bidders == 10  # From child
        assert merged.idr.enabled is True  # From parent
        assert merged.idr.exploration_rate == 0.1  # From parent


class TestConfigResolver:
    """Test ConfigResolver."""

    @pytest.fixture
    def resolver(self):
        """Create a fresh resolver for each test."""
        return ConfigResolver()

    @pytest.fixture
    def sample_publisher(self):
        """Create a sample publisher config."""
        return PublisherConfigV2(
            publisher_id="test-pub",
            name="Test Publisher",
            enabled=True,
            features=FeatureConfig(
                config_id="publisher:test-pub",
                config_level=ConfigLevel.PUBLISHER,
                idr=IDRSettings(max_bidders=12, exploration_rate=0.15),
            ),
            sites=[
                SiteConfig(
                    site_id="test-site",
                    domain="test.example.com",
                    name="Test Site",
                    features=FeatureConfig(
                        config_id="site:test-site",
                        config_level=ConfigLevel.SITE,
                        idr=IDRSettings(max_bidders=8),
                    ),
                    ad_units=[
                        AdUnitConfig(
                            unit_id="test-unit",
                            name="Test Unit",
                            sizes=[[300, 250]],
                            media_type="banner",
                            features=FeatureConfig(
                                config_id="ad_unit:test-unit",
                                config_level=ConfigLevel.AD_UNIT,
                                idr=IDRSettings(max_bidders=5),
                            ),
                        ),
                    ],
                ),
            ],
        )

    def test_resolve_global(self, resolver):
        """Test resolving global config."""
        resolved = resolver.resolve()

        assert resolved.config_level == ConfigLevel.GLOBAL
        assert "global" in resolved.resolution_chain
        assert resolved.max_bidders == 15  # Global default

    def test_resolve_publisher(self, resolver, sample_publisher):
        """Test resolving publisher config."""
        resolver.register_publisher(sample_publisher)

        resolved = resolver.resolve_for_publisher("test-pub")

        assert resolved.config_level == ConfigLevel.PUBLISHER
        assert "publisher:test-pub" in resolved.resolution_chain
        assert resolved.max_bidders == 12  # Publisher override
        assert resolved.exploration_rate == 0.15  # Publisher override

    def test_resolve_site(self, resolver, sample_publisher):
        """Test resolving site config."""
        resolver.register_publisher(sample_publisher)

        resolved = resolver.resolve_for_site("test-pub", "test-site")

        assert resolved.config_level == ConfigLevel.SITE
        assert "site:test-site" in resolved.resolution_chain
        assert resolved.max_bidders == 8  # Site override
        # Inherits from publisher
        assert resolved.exploration_rate == 0.15

    def test_resolve_ad_unit(self, resolver, sample_publisher):
        """Test resolving ad unit config."""
        resolver.register_publisher(sample_publisher)

        resolved = resolver.resolve_for_ad_unit("test-pub", "test-site", "test-unit")

        assert resolved.config_level == ConfigLevel.AD_UNIT
        assert "ad_unit:test-unit" in resolved.resolution_chain
        assert resolved.max_bidders == 5  # Ad unit override
        # Inherits from site -> publisher
        assert resolved.exploration_rate == 0.15

    def test_inheritance_chain(self, resolver, sample_publisher):
        """Test the full inheritance chain."""
        resolver.register_publisher(sample_publisher)

        resolved = resolver.resolve_for_ad_unit("test-pub", "test-site", "test-unit")

        # Verify the resolution chain
        assert resolved.resolution_chain == [
            "global",
            "publisher:test-pub",
            "site:test-site",
            "ad_unit:test-unit",
        ]

    def test_missing_publisher_returns_global(self, resolver):
        """Test that missing publisher falls back to global."""
        resolved = resolver.resolve_for_publisher("nonexistent")

        assert resolved.config_level == ConfigLevel.PUBLISHER
        assert resolved.max_bidders == 15  # Global default

    def test_cache_is_used(self, resolver, sample_publisher):
        """Test that resolved configs are cached."""
        resolver.register_publisher(sample_publisher)

        resolved1 = resolver.resolve_for_publisher("test-pub")
        resolved2 = resolver.resolve_for_publisher("test-pub")

        # Should be the same object due to caching
        assert resolved1 is resolved2

    def test_cache_cleared_on_update(self, resolver, sample_publisher):
        """Test that cache is cleared when config is updated."""
        resolver.register_publisher(sample_publisher)

        resolved1 = resolver.resolve_for_publisher("test-pub")

        # Update publisher
        sample_publisher.features.idr.max_bidders = 20
        resolver.register_publisher(sample_publisher)

        resolved2 = resolver.resolve_for_publisher("test-pub")

        # Should be different due to cache invalidation
        assert resolved2.max_bidders == 20


class TestResolvedConfig:
    """Test ResolvedConfig dataclass."""

    def test_to_dict(self):
        """Test ResolvedConfig serialization."""
        resolved = ResolvedConfig(
            config_id="test",
            config_level=ConfigLevel.PUBLISHER,
            resolution_chain=["global", "publisher:test"],
            max_bidders=10,
            exploration_rate=0.2,
        )

        data = resolved.to_dict()

        assert data["config_id"] == "test"
        assert data["config_level"] == "publisher"
        assert data["resolution_chain"] == ["global", "publisher:test"]
        assert data["idr"]["max_bidders"] == 10
        assert data["idr"]["exploration_rate"] == 0.2


class TestPublisherConfigV2:
    """Test PublisherConfigV2 dataclass."""

    def test_get_site(self):
        """Test getting a site by ID."""
        publisher = PublisherConfigV2(
            publisher_id="test-pub",
            name="Test Publisher",
            sites=[
                SiteConfig(site_id="site1", domain="site1.com"),
                SiteConfig(site_id="site2", domain="site2.com"),
            ],
        )

        site = publisher.get_site("site1")
        assert site is not None
        assert site.site_id == "site1"

        site = publisher.get_site("nonexistent")
        assert site is None

    def test_get_ad_unit(self):
        """Test getting an ad unit by site and unit ID."""
        publisher = PublisherConfigV2(
            publisher_id="test-pub",
            name="Test Publisher",
            sites=[
                SiteConfig(
                    site_id="site1",
                    domain="site1.com",
                    ad_units=[
                        AdUnitConfig(unit_id="unit1", name="Unit 1"),
                        AdUnitConfig(unit_id="unit2", name="Unit 2"),
                    ],
                ),
            ],
        )

        unit = publisher.get_ad_unit("site1", "unit1")
        assert unit is not None
        assert unit.unit_id == "unit1"

        unit = publisher.get_ad_unit("site1", "nonexistent")
        assert unit is None


class TestSelectorIntegration:
    """Test integration with PartnerSelector."""

    def test_selector_config_from_resolved(self):
        """Test creating SelectorConfig from ResolvedConfig."""
        from src.idr.selector.partner_selector import SelectorConfig

        resolved = ResolvedConfig(
            config_id="test",
            config_level=ConfigLevel.PUBLISHER,
            resolution_chain=["global", "publisher:test"],
            max_bidders=10,
            min_score_threshold=30.0,
            exploration_rate=0.2,
            exploration_enabled=True,
            exploration_slots=3,
            diversity_enabled=True,
            diversity_categories=["premium", "mid_tier"],
            bypass_enabled=False,
            shadow_mode=False,
        )

        config = SelectorConfig.from_resolved_config(resolved)

        assert config.max_bidders == 10
        assert config.min_score_threshold == 30.0
        assert config.exploration_rate == 0.2
        assert config.exploration_slots == 3
        assert config.diversity_enabled is True
        assert config.diversity_categories == ["premium", "mid_tier"]

    def test_create_selector_for_context(self):
        """Test creating a selector with hierarchical config."""
        from src.idr.selector.partner_selector import create_selector_for_context

        # Create resolver with config
        resolver = get_config_resolver()
        publisher = PublisherConfigV2(
            publisher_id="test-pub",
            name="Test Publisher",
            features=FeatureConfig(
                config_id="publisher:test-pub",
                config_level=ConfigLevel.PUBLISHER,
                idr=IDRSettings(max_bidders=8),
            ),
        )
        resolver.register_publisher(publisher)

        # Create selector
        selector = create_selector_for_context(publisher_id="test-pub")

        assert selector.config.max_bidders == 8


class TestConvenienceFunction:
    """Test the resolve_config convenience function."""

    def test_resolve_config_global(self):
        """Test resolve_config with no arguments."""
        resolved = resolve_config()
        assert resolved.config_level == ConfigLevel.GLOBAL

    def test_resolve_config_publisher(self):
        """Test resolve_config with publisher_id."""
        resolved = resolve_config(publisher_id="nonexistent")
        assert resolved.config_level == ConfigLevel.PUBLISHER

    def test_resolve_config_site(self):
        """Test resolve_config with publisher_id and site_id."""
        resolved = resolve_config(publisher_id="pub", site_id="site")
        assert resolved.config_level == ConfigLevel.SITE

    def test_resolve_config_ad_unit(self):
        """Test resolve_config with all IDs."""
        resolved = resolve_config(publisher_id="pub", site_id="site", unit_id="unit")
        assert resolved.config_level == ConfigLevel.AD_UNIT
