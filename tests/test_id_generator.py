"""Tests for the ID generator utilities."""

import re

from src.idr.utils.id_generator import (
    ALPHANUMERIC_CHARS,
    generate_ad_unit_id,
    generate_alphanumeric_id,
    generate_publisher_id,
    generate_site_id,
)


class TestGeneratePublisherId:
    """Test suite for generate_publisher_id function."""

    def test_default_format(self):
        """Test default publisher ID format."""
        result = generate_publisher_id()

        assert result.startswith("pub_")
        # Default length is 12 chars after prefix
        assert len(result) == 16  # "pub_" (4) + 12

    def test_custom_length(self):
        """Test custom length parameter."""
        result = generate_publisher_id(length=8)

        assert result.startswith("pub_")
        assert len(result) == 12  # "pub_" (4) + 8

    def test_custom_prefix(self):
        """Test custom prefix parameter."""
        result = generate_publisher_id(prefix="publisher_")

        assert result.startswith("publisher_")
        assert len(result) == 22  # "publisher_" (10) + 12

    def test_alphanumeric_only(self):
        """Test that only alphanumeric chars are used."""
        result = generate_publisher_id()

        # Remove prefix and check random part
        random_part = result[4:]  # Remove "pub_"
        for char in random_part:
            assert char in ALPHANUMERIC_CHARS

    def test_lowercase_only(self):
        """Test that only lowercase letters are used."""
        result = generate_publisher_id()

        # Remove prefix and check random part
        random_part = result[4:]  # Remove "pub_"
        assert (
            random_part.islower()
            or random_part.isdigit()
            or all(c.islower() or c.isdigit() for c in random_part)
        )

    def test_uniqueness(self):
        """Test that generated IDs are unique."""
        ids = [generate_publisher_id() for _ in range(100)]
        assert len(set(ids)) == 100

    def test_format_regex(self):
        """Test ID matches expected format pattern."""
        result = generate_publisher_id()

        pattern = r"^pub_[a-z0-9]{12}$"
        assert re.match(pattern, result)


class TestGenerateAlphanumericId:
    """Test suite for generate_alphanumeric_id function."""

    def test_default_length(self):
        """Test default ID length."""
        result = generate_alphanumeric_id()

        assert len(result) == 16

    def test_custom_length(self):
        """Test custom length parameter."""
        result = generate_alphanumeric_id(length=24)

        assert len(result) == 24

    def test_alphanumeric_only(self):
        """Test that only alphanumeric chars are used."""
        result = generate_alphanumeric_id()

        for char in result:
            assert char in ALPHANUMERIC_CHARS

    def test_no_prefix(self):
        """Test that no prefix is included."""
        result = generate_alphanumeric_id()

        # Should not start with any common prefix
        assert not result.startswith("pub_")
        assert not result.startswith("id_")

    def test_uniqueness(self):
        """Test that generated IDs are unique."""
        ids = [generate_alphanumeric_id() for _ in range(100)]
        assert len(set(ids)) == 100

    def test_format_regex(self):
        """Test ID matches expected format pattern."""
        result = generate_alphanumeric_id()

        pattern = r"^[a-z0-9]{16}$"
        assert re.match(pattern, result)


class TestGenerateSiteId:
    """Test suite for generate_site_id function."""

    def test_default_format(self):
        """Test default site ID format."""
        result = generate_site_id()

        assert result.startswith("site_")
        # Default length is 12 chars after prefix
        assert len(result) == 17  # "site_" (5) + 12

    def test_custom_length(self):
        """Test custom length parameter."""
        result = generate_site_id(length=8)

        assert result.startswith("site_")
        assert len(result) == 13  # "site_" (5) + 8

    def test_custom_prefix(self):
        """Test custom prefix parameter."""
        result = generate_site_id(prefix="s_")

        assert result.startswith("s_")
        assert len(result) == 14  # "s_" (2) + 12

    def test_alphanumeric_only(self):
        """Test that only alphanumeric chars are used."""
        result = generate_site_id()

        # Remove prefix and check random part
        random_part = result[5:]  # Remove "site_"
        for char in random_part:
            assert char in ALPHANUMERIC_CHARS

    def test_uniqueness(self):
        """Test that generated IDs are unique."""
        ids = [generate_site_id() for _ in range(100)]
        assert len(set(ids)) == 100

    def test_format_regex(self):
        """Test ID matches expected format pattern."""
        result = generate_site_id()

        pattern = r"^site_[a-z0-9]{12}$"
        assert re.match(pattern, result)


class TestGenerateAdUnitId:
    """Test suite for generate_ad_unit_id function."""

    def test_default_format(self):
        """Test default ad unit ID format."""
        result = generate_ad_unit_id()

        assert result.startswith("unit_")
        # Default length is 12 chars after prefix
        assert len(result) == 17  # "unit_" (5) + 12

    def test_custom_length(self):
        """Test custom length parameter."""
        result = generate_ad_unit_id(length=8)

        assert result.startswith("unit_")
        assert len(result) == 13  # "unit_" (5) + 8

    def test_custom_prefix(self):
        """Test custom prefix parameter."""
        result = generate_ad_unit_id(prefix="ad_")

        assert result.startswith("ad_")
        assert len(result) == 15  # "ad_" (3) + 12

    def test_alphanumeric_only(self):
        """Test that only alphanumeric chars are used."""
        result = generate_ad_unit_id()

        # Remove prefix and check random part
        random_part = result[5:]  # Remove "unit_"
        for char in random_part:
            assert char in ALPHANUMERIC_CHARS

    def test_uniqueness(self):
        """Test that generated IDs are unique."""
        ids = [generate_ad_unit_id() for _ in range(100)]
        assert len(set(ids)) == 100

    def test_format_regex(self):
        """Test ID matches expected format pattern."""
        result = generate_ad_unit_id()

        pattern = r"^unit_[a-z0-9]{12}$"
        assert re.match(pattern, result)
