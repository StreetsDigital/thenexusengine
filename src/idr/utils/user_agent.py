"""User Agent parsing utilities for device and browser detection."""

import re
from dataclasses import dataclass


@dataclass
class ParsedUserAgent:
    """Parsed user agent information."""

    browser: str
    browser_version: str | None
    os: str
    os_version: str | None
    device_type: str
    is_mobile: bool
    is_tablet: bool
    is_bot: bool


# Browser detection patterns (order matters - more specific first)
BROWSER_PATTERNS: list[tuple[str, re.Pattern]] = [
    ("edge", re.compile(r"Edg(?:e|A|iOS)?/(\d+)", re.I)),
    ("opera", re.compile(r"(?:Opera|OPR)/(\d+)", re.I)),
    ("samsung", re.compile(r"SamsungBrowser/(\d+)", re.I)),
    ("ucbrowser", re.compile(r"UCBrowser/(\d+)", re.I)),
    ("chrome", re.compile(r"(?:Chrome|CriOS)/(\d+)", re.I)),
    ("firefox", re.compile(r"(?:Firefox|FxiOS)/(\d+)", re.I)),
    ("safari", re.compile(r"Version/(\d+).*Safari", re.I)),
    ("ie", re.compile(r"(?:MSIE |Trident.*rv:)(\d+)", re.I)),
]

# OS detection patterns
OS_PATTERNS: list[tuple[str, re.Pattern]] = [
    ("ios", re.compile(r"(?:iPhone|iPad|iPod).*OS (\d+[_\.]\d+)", re.I)),
    ("android", re.compile(r"Android (\d+\.?\d*)", re.I)),
    ("windows", re.compile(r"Windows NT (\d+\.\d+)", re.I)),
    ("macos", re.compile(r"Mac OS X (\d+[_\.]\d+)", re.I)),
    ("linux", re.compile(r"Linux", re.I)),
    ("chromeos", re.compile(r"CrOS", re.I)),
]

# Mobile detection patterns
MOBILE_PATTERNS: list[re.Pattern] = [
    re.compile(r"Mobile|Android|iPhone|iPod|BlackBerry|IEMobile|Opera Mini", re.I),
]

# Tablet detection patterns
TABLET_PATTERNS: list[re.Pattern] = [
    re.compile(r"iPad|Android(?!.*Mobile)|Tablet", re.I),
]

# Bot detection patterns
BOT_PATTERNS: list[re.Pattern] = [
    re.compile(r"bot|crawl|spider|slurp|googlebot|bingbot|yandex|baidu", re.I),
]

# Windows version mapping
WINDOWS_VERSIONS: dict[str, str] = {
    "10.0": "10",
    "6.3": "8.1",
    "6.2": "8",
    "6.1": "7",
    "6.0": "Vista",
    "5.1": "XP",
}


def parse_user_agent(ua_string: str) -> ParsedUserAgent:
    """
    Parse a user agent string to extract browser, OS, and device info.

    Args:
        ua_string: The user agent string to parse

    Returns:
        ParsedUserAgent with extracted information
    """
    if not ua_string:
        return ParsedUserAgent(
            browser="unknown",
            browser_version=None,
            os="unknown",
            os_version=None,
            device_type="unknown",
            is_mobile=False,
            is_tablet=False,
            is_bot=False,
        )

    browser, browser_version = extract_browser(ua_string)
    os_name, os_version = extract_os(ua_string)
    is_mobile = _is_mobile(ua_string)
    is_tablet = _is_tablet(ua_string)
    is_bot = _is_bot(ua_string)

    # Determine device type
    if is_bot:
        device_type = "bot"
    elif is_tablet:
        device_type = "tablet"
    elif is_mobile:
        device_type = "mobile"
    else:
        device_type = "desktop"

    return ParsedUserAgent(
        browser=browser,
        browser_version=browser_version,
        os=os_name,
        os_version=os_version,
        device_type=device_type,
        is_mobile=is_mobile,
        is_tablet=is_tablet,
        is_bot=is_bot,
    )


def extract_browser(ua_string: str) -> tuple[str, str | None]:
    """
    Extract browser name and version from user agent.

    Args:
        ua_string: The user agent string

    Returns:
        Tuple of (browser_name, version) - version may be None
    """
    if not ua_string:
        return ("unknown", None)

    for browser_name, pattern in BROWSER_PATTERNS:
        match = pattern.search(ua_string)
        if match:
            version = match.group(1) if match.lastindex else None
            return (browser_name, version)

    return ("unknown", None)


def extract_os(ua_string: str) -> tuple[str, str | None]:
    """
    Extract operating system and version from user agent.

    Args:
        ua_string: The user agent string

    Returns:
        Tuple of (os_name, version) - version may be None
    """
    if not ua_string:
        return ("unknown", None)

    for os_name, pattern in OS_PATTERNS:
        match = pattern.search(ua_string)
        if match:
            version = None
            if match.lastindex:
                version = match.group(1).replace("_", ".")
                # Map Windows NT versions to friendly names
                if os_name == "windows" and version in WINDOWS_VERSIONS:
                    version = WINDOWS_VERSIONS[version]
            return (os_name, version)

    return ("unknown", None)


def _is_mobile(ua_string: str) -> bool:
    """Check if user agent indicates a mobile device."""
    return any(pattern.search(ua_string) for pattern in MOBILE_PATTERNS)


def _is_tablet(ua_string: str) -> bool:
    """Check if user agent indicates a tablet device."""
    return any(pattern.search(ua_string) for pattern in TABLET_PATTERNS)


def _is_bot(ua_string: str) -> bool:
    """Check if user agent indicates a bot/crawler."""
    return any(pattern.search(ua_string) for pattern in BOT_PATTERNS)
