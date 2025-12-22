"""IDR Constants and Configuration Values."""

# Bidder Categories for diversity selection
BIDDER_CATEGORIES: dict[str, str] = {
    # Premium SSPs
    "appnexus": "premium",
    "rubicon": "premium",
    "pubmatic": "premium",
    "openx": "premium",
    "ix": "premium",
    # Mid-tier SSPs
    "triplelift": "mid_tier",
    "sovrn": "mid_tier",
    "sharethrough": "mid_tier",
    "gumgum": "mid_tier",
    "33across": "mid_tier",
    "conversant": "mid_tier",
    "yieldmo": "mid_tier",
    "kargo": "mid_tier",
    # Video specialists
    "spotx": "video_specialist",
    "beachfront": "video_specialist",
    "unruly": "video_specialist",
    "telaria": "video_specialist",
    "springserve": "video_specialist",
    # Native specialists
    "teads": "native",
    "outbrain": "native",
    "taboola": "native",
    "nativo": "native",
    # Regional - EMEA
    "adform": "emea",
    "smartadserver": "emea",
    "improvedigital": "emea",
    "richaudience": "emea",
    # Regional - APAC
    "fluct": "apac",
    "mediafuse": "apac",
    # Mobile specialists
    "inmobi": "mobile",
    "smaato": "mobile",
    "verizonmedia": "mobile",
}

# ID Type Preferences by Bidder
# Order indicates preference priority
ID_PREFERENCES: dict[str, list[str]] = {
    "appnexus": ["uid2", "liveramp", "id5"],
    "rubicon": ["liveramp", "uid2", "id5"],
    "pubmatic": ["uid2", "id5", "sharedid"],
    "openx": ["uid2", "id5", "liveramp"],
    "ix": ["id5", "uid2", "liveramp"],
    "triplelift": ["uid2", "id5"],
    "sovrn": ["uid2", "id5", "sharedid"],
    "sharethrough": ["uid2", "id5"],
    "gumgum": ["uid2", "id5"],
    "33across": ["id5", "uid2", "33across_id"],
    "criteo": ["criteo"],  # Criteo prefers their own ID
    "teads": ["uid2", "id5"],
    "spotx": ["uid2", "liveramp"],
    "beachfront": ["uid2", "id5"],
    "unruly": ["uid2", "id5", "liveramp"],
    "adform": ["id5", "uid2"],
    "smartadserver": ["uid2", "id5"],
    "improvedigital": ["id5", "uid2", "liveramp"],
}

# Default ID preferences for bidders not explicitly listed
DEFAULT_ID_PREFERENCES: list[str] = ["uid2", "id5", "liveramp"]

# Common ad sizes
STANDARD_AD_SIZES: list[str] = [
    "300x250",  # Medium Rectangle
    "728x90",  # Leaderboard
    "160x600",  # Wide Skyscraper
    "300x600",  # Half Page
    "320x50",  # Mobile Leaderboard
    "320x100",  # Large Mobile Banner
    "970x250",  # Billboard
    "970x90",  # Super Leaderboard
    "300x50",  # Mobile Banner
    "468x60",  # Banner
    "336x280",  # Large Rectangle
    "250x250",  # Square
    "200x200",  # Small Square
    "120x600",  # Skyscraper
]

# Video ad sizes
VIDEO_AD_SIZES: list[str] = [
    "640x480",
    "640x360",
    "1280x720",
    "1920x1080",
]

# Device type mapping from OpenRTB device.devicetype
OPENRTB_DEVICE_TYPE_MAP: dict[int, str] = {
    1: "mobile",  # Mobile/Tablet
    2: "desktop",  # Personal Computer
    3: "ctv",  # Connected TV
    4: "mobile",  # Phone
    5: "tablet",  # Tablet
    6: "ctv",  # Connected Device
    7: "ctv",  # Set Top Box
}

# Connection type mapping from OpenRTB device.connectiontype
CONNECTION_TYPE_MAP: dict[int, str] = {
    0: "unknown",
    1: "ethernet",
    2: "wifi",
    3: "cellular_unknown",
    4: "2g",
    5: "3g",
    6: "4g",
    7: "5g",
}

# Position mapping from OpenRTB imp.banner.pos
POSITION_MAP: dict[int, str] = {
    0: "unknown",
    1: "atf",  # Above the fold
    2: "unknown",  # Deprecated
    3: "btf",  # Below the fold
    4: "unknown",  # Header
    5: "unknown",  # Footer
    6: "unknown",  # Sidebar
    7: "unknown",  # Full screen
}

# Scoring weights (should sum to 1.0)
SCORING_WEIGHTS: dict[str, float] = {
    "win_rate": 0.25,
    "bid_rate": 0.20,
    "cpm": 0.15,
    "floor_clearance": 0.15,
    "latency": 0.10,
    "recency": 0.10,
    "id_match": 0.05,
}

# Minimum sample size for reliable scoring
MIN_SAMPLE_SIZE: int = 100
COLD_START_THRESHOLD: int = 10000  # Auctions before full IDR activation
