"""Lookup Key model for hierarchical performance data retrieval."""

from dataclasses import dataclass


@dataclass(frozen=True)
class LookupKey:
    """
    Hierarchical lookup key for bidder performance data.

    Supports fallback from specific to broad:
    1. Exact match: country + device + format + size + publisher
    2. Broader: country + device + format + publisher
    3. Broadest: country + device + publisher
    4. Fallback: publisher only
    """

    country: str
    device_type: str
    ad_format: str
    ad_size: str | None = None
    publisher_id: str | None = None

    def to_redis_key(self, bidder_code: str) -> str:
        """Generate Redis key for this lookup."""
        parts = [
            f"score:{self.publisher_id or 'global'}",
            self.country,
            self.device_type,
            self.ad_format,
        ]
        if self.ad_size:
            parts.append(self.ad_size)
        parts.append(bidder_code)
        return ":".join(parts)

    def get_fallback_keys(self) -> list["LookupKey"]:
        """
        Generate list of fallback keys from most specific to least.

        Returns keys in order of specificity for hierarchical lookup.
        """
        keys = [self]  # Start with exact match

        # Without ad_size
        if self.ad_size:
            keys.append(
                LookupKey(
                    country=self.country,
                    device_type=self.device_type,
                    ad_format=self.ad_format,
                    ad_size=None,
                    publisher_id=self.publisher_id,
                )
            )

        # Without publisher (global stats)
        if self.publisher_id:
            keys.append(
                LookupKey(
                    country=self.country,
                    device_type=self.device_type,
                    ad_format=self.ad_format,
                    ad_size=self.ad_size,
                    publisher_id=None,
                )
            )

            # Global without ad_size
            if self.ad_size:
                keys.append(
                    LookupKey(
                        country=self.country,
                        device_type=self.device_type,
                        ad_format=self.ad_format,
                        ad_size=None,
                        publisher_id=None,
                    )
                )

        return keys

    def __str__(self) -> str:
        """String representation for logging."""
        parts = [self.country, self.device_type, self.ad_format]
        if self.ad_size:
            parts.append(self.ad_size)
        if self.publisher_id:
            parts.append(f"pub:{self.publisher_id}")
        return "/".join(parts)
