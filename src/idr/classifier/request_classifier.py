"""
Request Classifier for the Intelligent Demand Router (IDR).

Extracts and normalizes features from incoming OpenRTB 2.x bid requests
for use in bidder scoring and partner selection.
"""

from datetime import datetime, timezone
from typing import Any

from ..models.classified_request import (
    AdFormat,
    AdPosition,
    ClassifiedRequest,
    DeviceType,
)
from ..privacy.consent_models import ConsentSignals
from ..utils.constants import (
    CONNECTION_TYPE_MAP,
    OPENRTB_DEVICE_TYPE_MAP,
    POSITION_MAP,
)
from ..utils.id_generator import (
    generate_ad_unit_id,
    generate_publisher_id,
    generate_site_id,
)
from ..utils.user_agent import parse_user_agent


class RequestClassifier:
    """
    Classifies and normalizes OpenRTB bid requests.

    Extracts relevant features from raw OpenRTB requests and produces
    a ClassifiedRequest object suitable for bidder scoring.
    """

    def __init__(self, default_floor_currency: str = "USD"):
        """
        Initialize the classifier.

        Args:
            default_floor_currency: Default currency for floor prices
        """
        self.default_floor_currency = default_floor_currency

    def classify(self, ortb_request: dict[str, Any]) -> ClassifiedRequest:
        """
        Classify an OpenRTB bid request.

        Args:
            ortb_request: OpenRTB 2.x bid request dictionary

        Returns:
            ClassifiedRequest with extracted and normalized features
        """
        # Extract main sections
        imp = self._get_first_impression(ortb_request)
        device = ortb_request.get("device", {})
        site = ortb_request.get("site", {})
        app = ortb_request.get("app", {})
        user = ortb_request.get("user", {})
        regs = ortb_request.get("regs", {})
        ortb_request.get("source", {})

        # Use site or app for publisher info
        publisher_source = site if site else app

        # Parse user agent for device/browser info
        ua_string = device.get("ua", "")
        parsed_ua = parse_user_agent(ua_string)

        # Extract geo information
        geo = device.get("geo", {}) or ortb_request.get("geo", {})

        # Extract privacy/consent signals
        consent_signals = ConsentSignals.from_openrtb(ortb_request)

        # Build classified request
        return ClassifiedRequest(
            # Impression attributes
            impression_id=imp.get("id", ""),
            ad_unit_id=self._extract_ad_unit_id(imp),
            ad_format=self._determine_format(imp),
            ad_sizes=self._extract_sizes(imp),
            position=self._determine_position(imp),
            # Environment
            device_type=self._classify_device(device, parsed_ua),
            os=self._extract_os(device, parsed_ua),
            browser=parsed_ua.browser,
            connection_type=self._extract_connection_type(device),
            # Geo
            country=geo.get("country", "unknown").upper(),
            region=geo.get("region", ""),
            dma=geo.get("metro") or geo.get("dma"),
            # Publisher
            publisher_id=self._extract_publisher_id(publisher_source),
            site_id=self._extract_site_id(publisher_source),
            domain=self._extract_domain(site, app),
            page_type=self._extract_page_type(site),
            categories=self._extract_categories(site, app),
            # User
            user_ids=self._extract_user_ids(user, ortb_request),
            has_consent=self._check_consent(regs, user),
            consent_string=self._extract_consent_string(user, regs),
            # Privacy / Consent
            consent_signals=consent_signals,
            # Context
            timestamp=datetime.now(timezone.utc),
            # Floors
            floor_price=self._extract_floor_price(imp),
            floor_currency=imp.get("bidfloorcur", self.default_floor_currency),
        )

    def _get_first_impression(self, ortb_request: dict) -> dict:
        """Get the first impression from the request."""
        imps = ortb_request.get("imp", [])
        return imps[0] if imps else {}

    def _extract_ad_unit_id(self, imp: dict) -> str:
        """
        Extract ad unit ID from impression object (tagid field).

        If no tagid is found, auto-generates a unique
        alphanumeric ID with the format: unit_{random_string}
        """
        ad_unit_id = imp.get("tagid", "")

        # Auto-populate if no ad unit ID is provided
        if not ad_unit_id:
            ad_unit_id = generate_ad_unit_id()

        return ad_unit_id

    def _extract_site_id(self, publisher_source: dict) -> str:
        """
        Extract site ID from site or app object.

        If no site ID is found, auto-generates a unique
        alphanumeric ID with the format: site_{random_string}
        """
        site_id = publisher_source.get("id", "")

        # Auto-populate if no site ID is provided
        if not site_id:
            site_id = generate_site_id()

        return site_id

    def _determine_format(self, imp: dict) -> AdFormat:
        """
        Determine the ad format from impression object.

        Priority: video > native > audio > banner
        """
        if imp.get("video"):
            return AdFormat.VIDEO
        if imp.get("native"):
            return AdFormat.NATIVE
        if imp.get("audio"):
            return AdFormat.AUDIO
        return AdFormat.BANNER

    def _extract_sizes(self, imp: dict) -> list[str]:
        """
        Extract ad sizes from impression.

        Handles both banner format array and single w/h attributes.
        """
        sizes = []

        # Check banner object
        banner = imp.get("banner", {})
        if banner:
            # Check format array first
            formats = banner.get("format", [])
            for fmt in formats:
                w = fmt.get("w")
                h = fmt.get("h")
                if w and h:
                    sizes.append(f"{w}x{h}")

            # Fallback to single w/h
            if not sizes:
                w = banner.get("w")
                h = banner.get("h")
                if w and h:
                    sizes.append(f"{w}x{h}")

        # Check video for player size
        video = imp.get("video", {})
        if video and not sizes:
            w = video.get("w")
            h = video.get("h")
            if w and h:
                sizes.append(f"{w}x{h}")

        return sizes

    def _determine_position(self, imp: dict) -> AdPosition:
        """Determine ad position from impression."""
        banner = imp.get("banner", {})
        pos = banner.get("pos", 0)

        position_str = POSITION_MAP.get(pos, "unknown")
        try:
            return AdPosition(position_str)
        except ValueError:
            return AdPosition.UNKNOWN

    def _classify_device(self, device: dict, parsed_ua: Any) -> DeviceType:
        """
        Classify device type from device object and user agent.

        Priority: OpenRTB devicetype > user agent parsing
        """
        # Check OpenRTB device type first
        device_type_id = device.get("devicetype")
        if device_type_id is not None:
            device_type_str = OPENRTB_DEVICE_TYPE_MAP.get(device_type_id)
            if device_type_str:
                try:
                    return DeviceType(device_type_str)
                except ValueError:
                    pass

        # Fallback to user agent parsing
        if parsed_ua.is_tablet:
            return DeviceType.TABLET
        if parsed_ua.is_mobile:
            return DeviceType.MOBILE

        # Check OS hints
        os_lower = device.get("os", "").lower()
        if os_lower in ("ios", "android"):
            return DeviceType.MOBILE
        if os_lower in ("tvos", "tizen", "webos", "roku"):
            return DeviceType.CTV

        return DeviceType.DESKTOP

    def _extract_os(self, device: dict, parsed_ua: Any) -> str:
        """Extract operating system from device object or user agent."""
        # Prefer explicit OS field
        if device.get("os"):
            return device["os"].lower()

        # Fallback to user agent parsing
        return parsed_ua.os

    def _extract_connection_type(self, device: dict) -> str:
        """Extract connection type from device object."""
        conn_type_id = device.get("connectiontype")
        if conn_type_id is not None:
            return CONNECTION_TYPE_MAP.get(conn_type_id, "unknown")
        return "unknown"

    def _extract_publisher_id(self, publisher_source: dict) -> str:
        """
        Extract publisher ID from site or app object.

        If no publisher ID is found in the request, auto-generates
        a unique alphanumeric ID with the format: pub_{random_string}
        """
        publisher = publisher_source.get("publisher", {})
        publisher_id = publisher.get("id", "") or publisher_source.get("id", "")

        # Auto-populate if no publisher ID is provided
        if not publisher_id:
            publisher_id = generate_publisher_id()

        return publisher_id

    def _extract_domain(self, site: dict, app: dict) -> str:
        """Extract domain from site or bundle from app."""
        if site:
            # Prefer domain, then extract from page URL
            domain = site.get("domain", "")
            if not domain:
                page = site.get("page", "")
                domain = self._extract_domain_from_url(page)
            return domain

        if app:
            # Use bundle identifier for apps
            return app.get("bundle", "")

        return ""

    def _extract_domain_from_url(self, url: str) -> str:
        """Extract domain from a URL string."""
        if not url:
            return ""

        try:
            # Remove protocol
            if "://" in url:
                url = url.split("://")[1]
            # Remove path
            domain = url.split("/")[0]
            # Remove port
            domain = domain.split(":")[0]
            return domain
        except (IndexError, AttributeError):
            return ""

    def _extract_page_type(self, site: dict) -> str | None:
        """
        Extract page type from site object.

        Uses content.context or infers from page URL patterns.
        """
        content = site.get("content", {})
        if content.get("context"):
            return content["context"]

        # Try to infer from page URL
        page = site.get("page", "").lower()
        if "/article" in page or "/news" in page or "/post" in page:
            return "article"
        if page.endswith("/") or "home" in page:
            return "homepage"
        if "/category" in page or "/section" in page:
            return "section"
        if "/search" in page:
            return "search"
        if "/video" in page or "/watch" in page:
            return "video"

        return None

    def _extract_categories(self, site: dict, app: dict) -> list[str]:
        """Extract IAB categories from site or app."""
        categories = []

        # Site categories
        if site:
            cat = site.get("cat", [])
            if isinstance(cat, list):
                categories.extend(cat)
            pagecat = site.get("pagecat", [])
            if isinstance(pagecat, list):
                categories.extend(pagecat)
            sectioncat = site.get("sectioncat", [])
            if isinstance(sectioncat, list):
                categories.extend(sectioncat)

        # App categories
        if app:
            cat = app.get("cat", [])
            if isinstance(cat, list):
                categories.extend(cat)

        # Content categories
        content = (site or app or {}).get("content", {})
        if content:
            cat = content.get("cat", [])
            if isinstance(cat, list):
                categories.extend(cat)

        # Deduplicate while preserving order
        seen = set()
        unique_categories = []
        for cat in categories:
            if cat not in seen:
                seen.add(cat)
                unique_categories.append(cat)

        return unique_categories

    def _extract_user_ids(self, user: dict, ortb_request: dict) -> dict[str, str]:
        """
        Extract user IDs from various sources.

        Checks: user.ext.eids, user.id, source.ext.schain
        """
        user_ids = {}

        # Extract from Extended IDs (EIDs) - primary source
        ext = user.get("ext", {})
        eids = ext.get("eids", [])

        for eid in eids:
            source = eid.get("source", "")
            uids = eid.get("uids", [])

            # Map EID sources to ID types
            id_type = self._map_eid_source_to_type(source)
            if id_type and uids:
                # Take first UID
                uid = uids[0].get("id")
                if uid:
                    user_ids[id_type] = uid

        # Check for direct user ID
        if user.get("id") and "user_id" not in user_ids:
            user_ids["user_id"] = user["id"]

        # Check for buyer UID
        if user.get("buyeruid"):
            user_ids["buyer_uid"] = user["buyeruid"]

        return user_ids

    def _map_eid_source_to_type(self, source: str) -> str | None:
        """Map EID source domain to standardized ID type."""
        source_lower = source.lower()

        mapping = {
            "uidapi.com": "uid2",
            "id5-sync.com": "id5",
            "liveramp.com": "liveramp",
            "sharedid.org": "sharedid",
            "pubcid.org": "pubcid",
            "criteo.com": "criteo",
            "33across.com": "33across_id",
            "adserver.org": "ttd",  # The Trade Desk
            "intentiq.com": "intentiq",
            "liveintent.com": "liveintent",
            "netid.de": "netid",
            "zeotap.com": "zeotap",
        }

        for domain, id_type in mapping.items():
            if domain in source_lower:
                return id_type

        return None

    def _check_consent(self, regs: dict, user: dict) -> bool:
        """
        Check if user has provided consent.

        Returns True if GDPR doesn't apply or consent is given.
        """
        ext = regs.get("ext", {})

        # Check GDPR applicability
        gdpr = ext.get("gdpr")
        if gdpr == 0:
            # GDPR doesn't apply
            return True

        # Check if consent string exists
        user_ext = user.get("ext", {})
        consent = user_ext.get("consent")

        return bool(consent)

    def _extract_consent_string(self, user: dict, regs: dict) -> str | None:
        """Extract TCF consent string or GPP string."""
        user_ext = user.get("ext", {})

        # TCF consent string
        consent = user_ext.get("consent")
        if consent:
            return consent

        # GPP string
        regs_ext = regs.get("ext", {})
        gpp = regs_ext.get("gpp")
        if gpp:
            return gpp

        return None

    def _extract_floor_price(self, imp: dict) -> float | None:
        """Extract floor price from impression."""
        bidfloor = imp.get("bidfloor")

        if bidfloor is not None:
            try:
                return float(bidfloor)
            except (ValueError, TypeError):
                return None

        return None
