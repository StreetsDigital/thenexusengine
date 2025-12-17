"""
ID generation utilities for the IDR service.

Provides functions to generate unique identifiers for publishers
and other entities when not explicitly provided.
"""

import secrets
import string

# Character set for alphanumeric IDs (letters and numbers)
ALPHANUMERIC_CHARS = string.ascii_lowercase + string.digits


def generate_publisher_id(length: int = 12, prefix: str = "pub_") -> str:
    """
    Generate a random alphanumeric publisher ID.

    Creates a unique identifier suitable for publishers who don't provide
    an explicit ID in their OpenRTB requests.

    Args:
        length: Length of the random portion (default 12)
        prefix: Prefix for the ID (default "pub_")

    Returns:
        A publisher ID in format: {prefix}{random_alphanumeric}
        Example: "pub_a7b3x9k2m4n1"
    """
    random_part = "".join(
        secrets.choice(ALPHANUMERIC_CHARS) for _ in range(length)
    )
    return f"{prefix}{random_part}"


def generate_alphanumeric_id(length: int = 16) -> str:
    """
    Generate a random alphanumeric ID without prefix.

    Args:
        length: Length of the ID (default 16)

    Returns:
        A random alphanumeric string
        Example: "a7b3x9k2m4n1p5q8"
    """
    return "".join(secrets.choice(ALPHANUMERIC_CHARS) for _ in range(length))
