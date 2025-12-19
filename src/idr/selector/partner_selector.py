"""
Partner Selector for the Intelligent Demand Router (IDR).

Selects which bidders to include in the auction based on scores,
ensuring optimal partner selection with diversity and exploration.

Includes privacy filtering to ensure compliance with GDPR, CCPA, etc.

Supports hierarchical configuration resolution:
- Global defaults
- Publisher-level overrides
- Site-level overrides
- Ad unit-level overrides
"""

import random
from dataclasses import dataclass, field
from enum import Enum
from typing import TYPE_CHECKING, Any, Optional, Protocol

from ..models.bidder_score import BidderScore
from ..models.classified_request import AdFormat, ClassifiedRequest
from ..utils.constants import BIDDER_CATEGORIES

if TYPE_CHECKING:
    from ..config.config_resolver import ResolvedConfig
    from ..privacy.privacy_filter import PrivacyFilter


class SelectionReason(str, Enum):
    """Reason why a bidder was selected."""

    ANCHOR = "anchor"  # Top revenue performer
    HIGH_SCORE = "high_score"  # High score above threshold
    EXPLORATION = "exploration"  # Low confidence, exploring
    DIVERSITY = "diversity"  # Category diversity requirement
    EXPLORATION_SLOT = "exploration_slot"  # Reserved exploration slot
    BYPASS = "bypass"  # Bypass mode - all bidders selected


class ExclusionReason(str, Enum):
    """Reason why a bidder was excluded."""

    LOW_SCORE = "low_score"
    MAX_BIDDERS = "max_bidders"
    PRIVACY_NO_TCF = "privacy_no_tcf"
    PRIVACY_NO_VENDOR = "privacy_no_vendor"
    PRIVACY_NO_PURPOSE = "privacy_no_purpose"
    PRIVACY_CCPA_OPTOUT = "privacy_ccpa_optout"
    PRIVACY_COPPA = "privacy_coppa"
    PRIVACY_REGION = "privacy_region"
    PRIVACY_PERSONALIZATION = "privacy_personalization"


@dataclass
class SelectorConfig:
    """Configuration for the Partner Selector."""

    # Bypass/Shadow modes
    bypass_enabled: bool = False  # If True, select ALL bidders (no filtering)
    shadow_mode: bool = False  # If True, log decisions but return all bidders

    # Privacy settings
    privacy_enabled: bool = True  # Enable privacy filtering
    privacy_strict_mode: bool = False  # Strict mode: reject unknown GVL IDs

    # Selection limits
    max_bidders: int = 15  # Maximum partners per auction
    min_score_threshold: float = 25.0  # Minimum score to be considered

    # Exploration settings
    exploration_rate: float = 0.1  # 10% chance to include low-confidence
    exploration_slots: int = 2  # Reserved slots for data gathering
    low_confidence_threshold: float = 0.5  # Below this = low confidence
    exploration_confidence_threshold: float = 0.3  # For exploration slots

    # Anchor bidders
    anchor_bidder_count: int = 3  # Always-include top performers

    # Diversity settings
    diversity_enabled: bool = True  # Enforce category diversity
    diversity_categories: list[str] = field(
        default_factory=lambda: ["premium", "mid_tier", "video_specialist", "native"]
    )

    @classmethod
    def from_dict(cls, config: dict) -> "SelectorConfig":
        """Create config from dictionary."""
        return cls(
            bypass_enabled=config.get("bypass_enabled", False),
            shadow_mode=config.get("shadow_mode", False),
            privacy_enabled=config.get("privacy_enabled", True),
            privacy_strict_mode=config.get("privacy_strict_mode", False),
            max_bidders=config.get("max_bidders", 15),
            min_score_threshold=config.get("min_score_threshold", 25.0),
            exploration_rate=config.get("exploration_rate", 0.1),
            exploration_slots=config.get("exploration_slots", 2),
            anchor_bidder_count=config.get("anchor_bidder_count", 3),
            low_confidence_threshold=config.get("low_confidence_threshold", 0.5),
            exploration_confidence_threshold=config.get(
                "exploration_confidence_threshold", 0.3
            ),
            diversity_enabled=config.get("diversity_enabled", True),
            diversity_categories=config.get(
                "diversity_categories",
                ["premium", "mid_tier", "video_specialist", "native"],
            ),
        )

    @classmethod
    def from_resolved_config(cls, resolved: "ResolvedConfig") -> "SelectorConfig":
        """
        Create config from a hierarchically resolved configuration.

        This allows the selector to use configuration that has been
        resolved through the Global → Publisher → Site → Ad Unit chain.
        """
        return cls(
            bypass_enabled=resolved.bypass_enabled,
            shadow_mode=resolved.shadow_mode,
            privacy_enabled=resolved.privacy_enabled,
            privacy_strict_mode=resolved.privacy_strict_mode,
            max_bidders=resolved.max_bidders,
            min_score_threshold=resolved.min_score_threshold,
            exploration_rate=resolved.exploration_rate
            if resolved.exploration_enabled
            else 0.0,
            exploration_slots=resolved.exploration_slots
            if resolved.exploration_enabled
            else 0,
            anchor_bidder_count=resolved.anchor_bidder_count
            if resolved.anchor_bidders_enabled
            else 0,
            low_confidence_threshold=resolved.low_confidence_threshold,
            exploration_confidence_threshold=resolved.exploration_confidence_threshold,
            diversity_enabled=resolved.diversity_enabled,
            diversity_categories=resolved.diversity_categories,
        )

    @classmethod
    def bypass(cls) -> "SelectorConfig":
        """Create a bypass configuration that selects all bidders."""
        return cls(bypass_enabled=True)

    @classmethod
    def shadow(cls) -> "SelectorConfig":
        """Create a shadow mode configuration for testing."""
        return cls(shadow_mode=True)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        return {
            "bypass_enabled": self.bypass_enabled,
            "shadow_mode": self.shadow_mode,
            "privacy_enabled": self.privacy_enabled,
            "privacy_strict_mode": self.privacy_strict_mode,
            "max_bidders": self.max_bidders,
            "min_score_threshold": self.min_score_threshold,
            "exploration_rate": self.exploration_rate,
            "exploration_slots": self.exploration_slots,
            "anchor_bidder_count": self.anchor_bidder_count,
            "low_confidence_threshold": self.low_confidence_threshold,
            "exploration_confidence_threshold": self.exploration_confidence_threshold,
            "diversity_enabled": self.diversity_enabled,
            "diversity_categories": self.diversity_categories,
        }


@dataclass
class SelectedBidder:
    """A selected bidder with selection metadata."""

    bidder_code: str
    score: float
    confidence: float
    reason: SelectionReason
    category: str | None = None


@dataclass
class ExcludedBidder:
    """A bidder that was excluded with reason."""

    bidder_code: str
    reason: ExclusionReason
    details: str = ""


@dataclass
class SelectionResult:
    """
    Result of partner selection process.

    Contains selected bidders and metadata about the selection.
    """

    selected: list[SelectedBidder]
    excluded: list[str]
    total_candidates: int
    selection_time_ms: float = 0.0
    bypass_mode: bool = False  # True if bypass was active
    shadow_mode: bool = False  # True if shadow mode was active
    shadow_would_exclude: list[str] = field(
        default_factory=list
    )  # What would be excluded

    # Privacy filtering results
    privacy_filtered: list[ExcludedBidder] = field(default_factory=list)
    privacy_summary: dict = field(default_factory=dict)

    @property
    def selected_bidder_codes(self) -> list[str]:
        """Get list of selected bidder codes."""
        return [b.bidder_code for b in self.selected]

    @property
    def anchor_count(self) -> int:
        """Count of anchor bidders selected."""
        return sum(1 for b in self.selected if b.reason == SelectionReason.ANCHOR)

    @property
    def exploration_count(self) -> int:
        """Count of exploration bidders selected."""
        return sum(
            1
            for b in self.selected
            if b.reason
            in (SelectionReason.EXPLORATION, SelectionReason.EXPLORATION_SLOT)
        )

    @property
    def diversity_count(self) -> int:
        """Count of diversity bidders selected."""
        return sum(1 for b in self.selected if b.reason == SelectionReason.DIVERSITY)

    @property
    def privacy_filtered_count(self) -> int:
        """Count of bidders filtered for privacy reasons."""
        return len(self.privacy_filtered)

    @property
    def is_filtered(self) -> bool:
        """Check if IDR actually filtered any bidders."""
        return not self.bypass_mode and not self.shadow_mode and len(self.excluded) > 0

    def to_dict(self) -> dict:
        """Convert to dictionary representation."""
        result = {
            "selected": [
                {
                    "bidder_code": b.bidder_code,
                    "score": round(b.score, 2),
                    "confidence": round(b.confidence, 3),
                    "reason": b.reason.value,
                    "category": b.category,
                }
                for b in self.selected
            ],
            "excluded": self.excluded,
            "total_candidates": self.total_candidates,
            "selection_time_ms": round(self.selection_time_ms, 2),
            "bypass_mode": self.bypass_mode,
            "shadow_mode": self.shadow_mode,
            "summary": {
                "selected_count": len(self.selected),
                "anchor_count": self.anchor_count,
                "exploration_count": self.exploration_count,
                "diversity_count": self.diversity_count,
                "privacy_filtered_count": self.privacy_filtered_count,
            },
        }

        # Include shadow analysis if in shadow mode
        if self.shadow_mode and self.shadow_would_exclude:
            result["shadow_analysis"] = {
                "would_exclude": self.shadow_would_exclude,
                "would_exclude_count": len(self.shadow_would_exclude),
            }

        # Include privacy filtering details
        if self.privacy_filtered:
            result["privacy"] = {
                "filtered": [
                    {
                        "bidder_code": b.bidder_code,
                        "reason": b.reason.value,
                        "details": b.details,
                    }
                    for b in self.privacy_filtered
                ],
                "summary": self.privacy_summary,
            }

        return result


class AnchorBidderProvider(Protocol):
    """Protocol for providing anchor bidders."""

    def get_top_bidders_by_revenue(
        self, publisher_id: str, limit: int = 3
    ) -> list[str]:
        """Get top revenue-generating bidders for a publisher."""
        ...


class PartnerSelector:
    """
    Selects which bidders to include in an auction.

    Selection strategy:
    0. Filter bidders based on privacy consent (GDPR, CCPA, COPPA)
    1. Always include "anchor" bidders (top by historical revenue)
    2. Add high-confidence high-scorers above threshold
    3. Handle low-confidence bidders with exploration rate
    4. Ensure category diversity
    5. Fill remaining slots with exploration candidates
    """

    def __init__(
        self,
        config: SelectorConfig | None = None,
        anchor_provider: AnchorBidderProvider | None = None,
        privacy_filter: Optional["PrivacyFilter"] = None,
        random_seed: int | None = None,
    ):
        """
        Initialize the partner selector.

        Args:
            config: Selector configuration
            anchor_provider: Provider for anchor bidder data
            privacy_filter: Privacy filter for GDPR/CCPA compliance
            random_seed: Optional seed for reproducible selection
        """
        self.config = config or SelectorConfig()
        self.anchor_provider = anchor_provider
        self.privacy_filter = privacy_filter
        self._rng = random.Random(random_seed)

    def select_partners(
        self,
        scores: list[BidderScore],
        request: ClassifiedRequest,
    ) -> SelectionResult:
        """
        Select partners for the auction.

        Args:
            scores: List of bidder scores
            request: Classified bid request

        Returns:
            SelectionResult with selected bidders and metadata
        """
        import time

        start_time = time.perf_counter()

        # Sort by score descending
        ranked = sorted(scores, key=lambda x: x.total_score, reverse=True)
        score_lookup = {s.bidder_code: s for s in ranked}

        # Apply privacy filtering first
        privacy_filtered: list[ExcludedBidder] = []
        privacy_summary: dict = {}

        if (
            self.config.privacy_enabled
            and self.privacy_filter
            and request.consent_signals
        ):
            ranked, privacy_filtered, privacy_summary = self._apply_privacy_filter(
                ranked, request
            )
            # Rebuild score lookup after filtering
            score_lookup = {s.bidder_code: s for s in ranked}

        # BYPASS MODE: Select all bidders, no filtering (except privacy)
        if self.config.bypass_enabled:
            selected = [
                SelectedBidder(
                    bidder_code=score.bidder_code,
                    score=score.total_score,
                    confidence=score.confidence,
                    reason=SelectionReason.BYPASS,
                    category=BIDDER_CATEGORIES.get(score.bidder_code),
                )
                for score in ranked
            ]
            selection_time = (time.perf_counter() - start_time) * 1000

            return SelectionResult(
                selected=selected,
                excluded=[],
                total_candidates=len(scores),
                selection_time_ms=selection_time,
                bypass_mode=True,
                privacy_filtered=privacy_filtered,
                privacy_summary=privacy_summary,
            )

        # Normal selection process
        bidder_codes_available = {s.bidder_code for s in ranked}
        selected: list[SelectedBidder] = []
        selected_codes: set[str] = set()

        # 1. Always include anchor bidders
        self._add_anchor_bidders(
            request.publisher_id,
            bidder_codes_available,
            score_lookup,
            selected,
            selected_codes,
        )

        # 2. Add high-confidence high-scorers
        self._add_high_scorers(
            ranked,
            selected,
            selected_codes,
        )

        # 3. Ensure category diversity
        if self.config.diversity_enabled:
            self._ensure_diversity(
                ranked,
                request,
                selected,
                selected_codes,
            )

        # 4. Add exploration slots
        self._add_exploration_slots(
            ranked,
            selected,
            selected_codes,
        )

        # Build excluded list
        excluded = [
            s.bidder_code for s in ranked if s.bidder_code not in selected_codes
        ]

        selection_time = (time.perf_counter() - start_time) * 1000

        # SHADOW MODE: Return all bidders but track what would be excluded
        if self.config.shadow_mode:
            # Add the excluded bidders back as BYPASS
            shadow_selected = list(selected)  # Keep original selections
            for bidder_code in excluded:
                score = score_lookup[bidder_code]
                shadow_selected.append(
                    SelectedBidder(
                        bidder_code=bidder_code,
                        score=score.total_score,
                        confidence=score.confidence,
                        reason=SelectionReason.BYPASS,
                        category=BIDDER_CATEGORIES.get(bidder_code),
                    )
                )

            return SelectionResult(
                selected=shadow_selected,
                excluded=[],  # Nothing actually excluded
                total_candidates=len(scores),
                selection_time_ms=selection_time,
                shadow_mode=True,
                shadow_would_exclude=excluded,  # What WOULD be excluded
                privacy_filtered=privacy_filtered,
                privacy_summary=privacy_summary,
            )

        return SelectionResult(
            selected=selected,
            excluded=excluded,
            total_candidates=len(scores),
            selection_time_ms=selection_time,
            privacy_filtered=privacy_filtered,
            privacy_summary=privacy_summary,
        )

    def _add_anchor_bidders(
        self,
        publisher_id: str,
        available: set[str],
        score_lookup: dict[str, BidderScore],
        selected: list[SelectedBidder],
        selected_codes: set[str],
    ) -> None:
        """Add anchor bidders (top revenue performers)."""
        anchor_bidders = self._get_anchor_bidders(publisher_id)

        for bidder_code in anchor_bidders:
            if bidder_code in available and bidder_code not in selected_codes:
                score = score_lookup.get(bidder_code)
                if score:
                    selected.append(
                        SelectedBidder(
                            bidder_code=bidder_code,
                            score=score.total_score,
                            confidence=score.confidence,
                            reason=SelectionReason.ANCHOR,
                            category=BIDDER_CATEGORIES.get(bidder_code),
                        )
                    )
                    selected_codes.add(bidder_code)

    def _add_high_scorers(
        self,
        ranked: list[BidderScore],
        selected: list[SelectedBidder],
        selected_codes: set[str],
    ) -> None:
        """Add high-confidence high-scorers."""
        for score in ranked:
            if score.bidder_code in selected_codes:
                continue

            if len(selected) >= self.config.max_bidders:
                break

            # Must meet minimum score threshold
            if score.total_score < self.config.min_score_threshold:
                continue

            # Handle low confidence bidders
            if score.confidence < self.config.low_confidence_threshold:
                # Include with exploration_rate probability
                if self._rng.random() > self.config.exploration_rate:
                    continue
                reason = SelectionReason.EXPLORATION
            else:
                reason = SelectionReason.HIGH_SCORE

            selected.append(
                SelectedBidder(
                    bidder_code=score.bidder_code,
                    score=score.total_score,
                    confidence=score.confidence,
                    reason=reason,
                    category=BIDDER_CATEGORIES.get(score.bidder_code),
                )
            )
            selected_codes.add(score.bidder_code)

    def _ensure_diversity(
        self,
        ranked: list[BidderScore],
        request: ClassifiedRequest,
        selected: list[SelectedBidder],
        selected_codes: set[str],
    ) -> None:
        """Ensure representation from different SSP categories."""
        categories_present = {
            BIDDER_CATEGORIES.get(b.bidder_code)
            for b in selected
            if BIDDER_CATEGORIES.get(b.bidder_code)
        }

        # Determine which categories to enforce based on request
        required_categories = self._get_required_categories(request)

        for category in required_categories:
            if category in categories_present:
                continue

            if len(selected) >= self.config.max_bidders:
                break

            # Find best scorer in this category
            for score in ranked:
                bidder_category = BIDDER_CATEGORIES.get(score.bidder_code)
                if (
                    bidder_category == category
                    and score.bidder_code not in selected_codes
                ):
                    selected.append(
                        SelectedBidder(
                            bidder_code=score.bidder_code,
                            score=score.total_score,
                            confidence=score.confidence,
                            reason=SelectionReason.DIVERSITY,
                            category=category,
                        )
                    )
                    selected_codes.add(score.bidder_code)
                    categories_present.add(category)
                    break

    def _get_required_categories(self, request: ClassifiedRequest) -> list[str]:
        """
        Get required categories based on request attributes.

        For video requests, include video_specialist.
        For native requests, include native.
        Always include premium and mid_tier.
        """
        categories = ["premium", "mid_tier"]

        if request.ad_format == AdFormat.VIDEO:
            categories.append("video_specialist")
        elif request.ad_format == AdFormat.NATIVE:
            categories.append("native")

        # Filter to only configured categories
        return [c for c in categories if c in self.config.diversity_categories]

    def _add_exploration_slots(
        self,
        ranked: list[BidderScore],
        selected: list[SelectedBidder],
        selected_codes: set[str],
    ) -> None:
        """Add exploration slots for data gathering."""
        if len(selected) >= self.config.max_bidders:
            return

        # Find exploration candidates (low confidence, not already selected)
        exploration_candidates = [
            score
            for score in ranked
            if score.bidder_code not in selected_codes
            and score.confidence < self.config.exploration_confidence_threshold
        ]

        if not exploration_candidates:
            return

        # Calculate how many exploration slots to fill
        exploration_count = min(
            self.config.exploration_slots,
            self.config.max_bidders - len(selected),
            len(exploration_candidates),
        )

        if exploration_count <= 0:
            return

        # Randomly select from candidates
        chosen = self._rng.sample(exploration_candidates, exploration_count)

        for score in chosen:
            selected.append(
                SelectedBidder(
                    bidder_code=score.bidder_code,
                    score=score.total_score,
                    confidence=score.confidence,
                    reason=SelectionReason.EXPLORATION_SLOT,
                    category=BIDDER_CATEGORIES.get(score.bidder_code),
                )
            )
            selected_codes.add(score.bidder_code)

    def _get_anchor_bidders(self, publisher_id: str) -> list[str]:
        """
        Get anchor bidders for a publisher.

        Falls back to default premium bidders if no provider.
        """
        if self.anchor_provider:
            return self.anchor_provider.get_top_bidders_by_revenue(
                publisher_id,
                limit=self.config.anchor_bidder_count,
            )

        # Default anchor bidders when no provider
        return ["rubicon", "appnexus", "pubmatic"][: self.config.anchor_bidder_count]

    def _apply_privacy_filter(
        self,
        ranked: list[BidderScore],
        request: ClassifiedRequest,
    ) -> tuple[list[BidderScore], list[ExcludedBidder], dict]:
        """
        Apply privacy filtering to bidders based on consent signals.

        Args:
            ranked: List of scored bidders
            request: Classified request with consent signals

        Returns:
            Tuple of (filtered_bidders, excluded_bidders, summary)
        """
        from ..privacy.privacy_filter import FilterReason

        if not self.privacy_filter or not request.consent_signals:
            return ranked, [], {}

        # Get bidder codes
        bidder_codes = [s.bidder_code for s in ranked]

        # Apply privacy filter
        allowed_codes, filter_results = self.privacy_filter.filter_bidders(
            bidder_codes, request.consent_signals
        )

        # Build excluded list with reasons
        excluded: list[ExcludedBidder] = []
        allowed_set = set(allowed_codes)

        # Map FilterReason to ExclusionReason
        reason_map = {
            FilterReason.NO_TCF_CONSENT: ExclusionReason.PRIVACY_NO_TCF,
            FilterReason.NO_VENDOR_CONSENT: ExclusionReason.PRIVACY_NO_VENDOR,
            FilterReason.NO_PURPOSE_CONSENT: ExclusionReason.PRIVACY_NO_PURPOSE,
            FilterReason.CCPA_OPT_OUT: ExclusionReason.PRIVACY_CCPA_OPTOUT,
            FilterReason.COPPA_RESTRICTED: ExclusionReason.PRIVACY_COPPA,
            FilterReason.REGION_BLOCKED: ExclusionReason.PRIVACY_REGION,
            FilterReason.PERSONALIZATION_DENIED: ExclusionReason.PRIVACY_PERSONALIZATION,
        }

        for result in filter_results:
            if not result.allowed:
                exclusion_reason = reason_map.get(
                    result.reason, ExclusionReason.PRIVACY_NO_TCF
                )
                excluded.append(
                    ExcludedBidder(
                        bidder_code=result.bidder_code,
                        reason=exclusion_reason,
                        details=result.details,
                    )
                )

        # Filter the ranked list
        filtered_ranked = [s for s in ranked if s.bidder_code in allowed_set]

        # Get summary from privacy filter
        summary = self.privacy_filter.get_privacy_summary(
            request.consent_signals, filter_results
        )

        return filtered_ranked, excluded, summary


class MockAnchorProvider:
    """Mock anchor provider for testing."""

    def __init__(self, anchors_by_publisher: dict[str, list[str]] | None = None):
        """Initialize with optional publisher-specific anchors."""
        self._anchors = anchors_by_publisher or {}
        self._default = ["rubicon", "appnexus", "pubmatic"]

    def set_anchors(self, publisher_id: str, bidders: list[str]) -> None:
        """Set anchor bidders for a publisher."""
        self._anchors[publisher_id] = bidders

    def get_top_bidders_by_revenue(
        self, publisher_id: str, limit: int = 3
    ) -> list[str]:
        """Get anchor bidders for publisher."""
        bidders = self._anchors.get(publisher_id, self._default)
        return bidders[:limit]


def create_selector_for_context(
    publisher_id: str | None = None,
    site_id: str | None = None,
    unit_id: str | None = None,
    anchor_provider: AnchorBidderProvider | None = None,
    privacy_filter: Optional["PrivacyFilter"] = None,
    random_seed: int | None = None,
) -> PartnerSelector:
    """
    Create a PartnerSelector with hierarchically resolved configuration.

    This is the recommended way to create a selector when you need
    configuration to be resolved through the hierarchy:
    Global → Publisher → Site → Ad Unit

    Args:
        publisher_id: Publisher ID for config resolution
        site_id: Site ID for config resolution (requires publisher_id)
        unit_id: Ad unit ID for config resolution (requires site_id)
        anchor_provider: Provider for anchor bidder data
        privacy_filter: Privacy filter for GDPR/CCPA compliance
        random_seed: Optional seed for reproducible selection

    Returns:
        PartnerSelector configured with resolved settings

    Example:
        # Get selector with global defaults
        selector = create_selector_for_context()

        # Get selector with publisher-specific settings
        selector = create_selector_for_context(publisher_id="pub123")

        # Get selector with site-specific settings
        selector = create_selector_for_context(
            publisher_id="pub123",
            site_id="site456"
        )

        # Get selector with ad unit-specific settings
        selector = create_selector_for_context(
            publisher_id="pub123",
            site_id="site456",
            unit_id="unit789"
        )
    """
    from ..config.config_resolver import resolve_config

    resolved = resolve_config(publisher_id, site_id, unit_id)
    config = SelectorConfig.from_resolved_config(resolved)

    # Use custom anchor bidders if specified in config
    effective_anchor_provider = anchor_provider
    if resolved.custom_anchor_bidders:
        effective_anchor_provider = MockAnchorProvider(
            {publisher_id or "default": resolved.custom_anchor_bidders}
        )

    return PartnerSelector(
        config=config,
        anchor_provider=effective_anchor_provider,
        privacy_filter=privacy_filter,
        random_seed=random_seed,
    )


def select_partners_for_request(
    scores: list[BidderScore],
    request: ClassifiedRequest,
    anchor_provider: AnchorBidderProvider | None = None,
    privacy_filter: Optional["PrivacyFilter"] = None,
) -> SelectionResult:
    """
    Select partners using hierarchically resolved configuration.

    This convenience function automatically resolves configuration based
    on the request's publisher_id, site_id, and ad unit information.

    Args:
        scores: List of bidder scores
        request: Classified bid request
        anchor_provider: Provider for anchor bidder data
        privacy_filter: Privacy filter for GDPR/CCPA compliance

    Returns:
        SelectionResult with selected bidders and metadata
    """
    # Extract context from request
    publisher_id = request.publisher_id if hasattr(request, "publisher_id") else None
    site_id = request.site_id if hasattr(request, "site_id") else None

    # Ad unit ID might be in different places depending on request structure
    unit_id = None
    if hasattr(request, "ad_unit_id"):
        unit_id = request.ad_unit_id
    elif hasattr(request, "impression_id"):
        unit_id = request.impression_id

    selector = create_selector_for_context(
        publisher_id=publisher_id,
        site_id=site_id,
        unit_id=unit_id,
        anchor_provider=anchor_provider,
        privacy_filter=privacy_filter,
    )

    return selector.select_partners(scores, request)
