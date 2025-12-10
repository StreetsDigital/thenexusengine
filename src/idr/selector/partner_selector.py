"""
Partner Selector for the Intelligent Demand Router (IDR).

Selects which bidders to include in the auction based on scores,
ensuring optimal partner selection with diversity and exploration.
"""

import random
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional, Protocol

from ..models.bidder_score import BidderScore
from ..models.classified_request import AdFormat, ClassifiedRequest
from ..utils.constants import BIDDER_CATEGORIES


class SelectionReason(str, Enum):
    """Reason why a bidder was selected."""
    ANCHOR = 'anchor'                    # Top revenue performer
    HIGH_SCORE = 'high_score'            # High score above threshold
    EXPLORATION = 'exploration'          # Low confidence, exploring
    DIVERSITY = 'diversity'              # Category diversity requirement
    EXPLORATION_SLOT = 'exploration_slot'  # Reserved exploration slot
    BYPASS = 'bypass'                    # Bypass mode - all bidders selected


@dataclass
class SelectorConfig:
    """Configuration for the Partner Selector."""

    # Bypass/Shadow modes
    bypass_enabled: bool = False         # If True, select ALL bidders (no filtering)
    shadow_mode: bool = False            # If True, log decisions but return all bidders

    # Selection limits
    max_bidders: int = 15                # Maximum partners per auction
    min_score_threshold: float = 25.0    # Minimum score to be considered

    # Exploration settings
    exploration_rate: float = 0.1        # 10% chance to include low-confidence
    exploration_slots: int = 2           # Reserved slots for data gathering
    low_confidence_threshold: float = 0.5  # Below this = low confidence
    exploration_confidence_threshold: float = 0.3  # For exploration slots

    # Anchor bidders
    anchor_bidder_count: int = 3         # Always-include top performers

    # Diversity settings
    diversity_enabled: bool = True       # Enforce category diversity
    diversity_categories: list[str] = field(
        default_factory=lambda: ['premium', 'mid_tier', 'video_specialist', 'native']
    )

    @classmethod
    def from_dict(cls, config: dict) -> 'SelectorConfig':
        """Create config from dictionary."""
        return cls(
            bypass_enabled=config.get('bypass_enabled', False),
            shadow_mode=config.get('shadow_mode', False),
            max_bidders=config.get('max_bidders', 15),
            min_score_threshold=config.get('min_score_threshold', 25.0),
            exploration_rate=config.get('exploration_rate', 0.1),
            exploration_slots=config.get('exploration_slots', 2),
            anchor_bidder_count=config.get('anchor_bidder_count', 3),
            low_confidence_threshold=config.get('low_confidence_threshold', 0.5),
            exploration_confidence_threshold=config.get(
                'exploration_confidence_threshold', 0.3
            ),
            diversity_enabled=config.get('diversity_enabled', True),
            diversity_categories=config.get(
                'diversity_categories',
                ['premium', 'mid_tier', 'video_specialist', 'native']
            ),
        )

    @classmethod
    def bypass(cls) -> 'SelectorConfig':
        """Create a bypass configuration that selects all bidders."""
        return cls(bypass_enabled=True)

    @classmethod
    def shadow(cls) -> 'SelectorConfig':
        """Create a shadow mode configuration for testing."""
        return cls(shadow_mode=True)


@dataclass
class SelectedBidder:
    """A selected bidder with selection metadata."""

    bidder_code: str
    score: float
    confidence: float
    reason: SelectionReason
    category: Optional[str] = None


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
    bypass_mode: bool = False            # True if bypass was active
    shadow_mode: bool = False            # True if shadow mode was active
    shadow_would_exclude: list[str] = field(default_factory=list)  # What would be excluded

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
            1 for b in self.selected
            if b.reason in (SelectionReason.EXPLORATION, SelectionReason.EXPLORATION_SLOT)
        )

    @property
    def diversity_count(self) -> int:
        """Count of diversity bidders selected."""
        return sum(1 for b in self.selected if b.reason == SelectionReason.DIVERSITY)

    @property
    def is_filtered(self) -> bool:
        """Check if IDR actually filtered any bidders."""
        return not self.bypass_mode and not self.shadow_mode and len(self.excluded) > 0

    def to_dict(self) -> dict:
        """Convert to dictionary representation."""
        result = {
            'selected': [
                {
                    'bidder_code': b.bidder_code,
                    'score': round(b.score, 2),
                    'confidence': round(b.confidence, 3),
                    'reason': b.reason.value,
                    'category': b.category,
                }
                for b in self.selected
            ],
            'excluded': self.excluded,
            'total_candidates': self.total_candidates,
            'selection_time_ms': round(self.selection_time_ms, 2),
            'bypass_mode': self.bypass_mode,
            'shadow_mode': self.shadow_mode,
            'summary': {
                'selected_count': len(self.selected),
                'anchor_count': self.anchor_count,
                'exploration_count': self.exploration_count,
                'diversity_count': self.diversity_count,
            },
        }

        # Include shadow analysis if in shadow mode
        if self.shadow_mode and self.shadow_would_exclude:
            result['shadow_analysis'] = {
                'would_exclude': self.shadow_would_exclude,
                'would_exclude_count': len(self.shadow_would_exclude),
            }

        return result


class AnchorBidderProvider(Protocol):
    """Protocol for providing anchor bidders."""

    def get_top_bidders_by_revenue(
        self,
        publisher_id: str,
        limit: int = 3
    ) -> list[str]:
        """Get top revenue-generating bidders for a publisher."""
        ...


class PartnerSelector:
    """
    Selects which bidders to include in an auction.

    Selection strategy:
    1. Always include "anchor" bidders (top by historical revenue)
    2. Add high-confidence high-scorers above threshold
    3. Handle low-confidence bidders with exploration rate
    4. Ensure category diversity
    5. Fill remaining slots with exploration candidates
    """

    def __init__(
        self,
        config: Optional[SelectorConfig] = None,
        anchor_provider: Optional[AnchorBidderProvider] = None,
        random_seed: Optional[int] = None,
    ):
        """
        Initialize the partner selector.

        Args:
            config: Selector configuration
            anchor_provider: Provider for anchor bidder data
            random_seed: Optional seed for reproducible selection
        """
        self.config = config or SelectorConfig()
        self.anchor_provider = anchor_provider
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

        # BYPASS MODE: Select all bidders, no filtering
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
            s.bidder_code for s in ranked
            if s.bidder_code not in selected_codes
        ]

        selection_time = (time.perf_counter() - start_time) * 1000

        # SHADOW MODE: Return all bidders but track what would be excluded
        if self.config.shadow_mode:
            # Add the excluded bidders back as BYPASS
            shadow_selected = list(selected)  # Keep original selections
            for bidder_code in excluded:
                score = score_lookup[bidder_code]
                shadow_selected.append(SelectedBidder(
                    bidder_code=bidder_code,
                    score=score.total_score,
                    confidence=score.confidence,
                    reason=SelectionReason.BYPASS,
                    category=BIDDER_CATEGORIES.get(bidder_code),
                ))

            return SelectionResult(
                selected=shadow_selected,
                excluded=[],  # Nothing actually excluded
                total_candidates=len(scores),
                selection_time_ms=selection_time,
                shadow_mode=True,
                shadow_would_exclude=excluded,  # What WOULD be excluded
            )

        return SelectionResult(
            selected=selected,
            excluded=excluded,
            total_candidates=len(scores),
            selection_time_ms=selection_time,
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
                    selected.append(SelectedBidder(
                        bidder_code=bidder_code,
                        score=score.total_score,
                        confidence=score.confidence,
                        reason=SelectionReason.ANCHOR,
                        category=BIDDER_CATEGORIES.get(bidder_code),
                    ))
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

            selected.append(SelectedBidder(
                bidder_code=score.bidder_code,
                score=score.total_score,
                confidence=score.confidence,
                reason=reason,
                category=BIDDER_CATEGORIES.get(score.bidder_code),
            ))
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
                if bidder_category == category and score.bidder_code not in selected_codes:
                    selected.append(SelectedBidder(
                        bidder_code=score.bidder_code,
                        score=score.total_score,
                        confidence=score.confidence,
                        reason=SelectionReason.DIVERSITY,
                        category=category,
                    ))
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
        categories = ['premium', 'mid_tier']

        if request.ad_format == AdFormat.VIDEO:
            categories.append('video_specialist')
        elif request.ad_format == AdFormat.NATIVE:
            categories.append('native')

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
            score for score in ranked
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
            selected.append(SelectedBidder(
                bidder_code=score.bidder_code,
                score=score.total_score,
                confidence=score.confidence,
                reason=SelectionReason.EXPLORATION_SLOT,
                category=BIDDER_CATEGORIES.get(score.bidder_code),
            ))
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
        return ['rubicon', 'appnexus', 'pubmatic'][:self.config.anchor_bidder_count]


class MockAnchorProvider:
    """Mock anchor provider for testing."""

    def __init__(self, anchors_by_publisher: Optional[dict[str, list[str]]] = None):
        """Initialize with optional publisher-specific anchors."""
        self._anchors = anchors_by_publisher or {}
        self._default = ['rubicon', 'appnexus', 'pubmatic']

    def set_anchors(self, publisher_id: str, bidders: list[str]) -> None:
        """Set anchor bidders for a publisher."""
        self._anchors[publisher_id] = bidders

    def get_top_bidders_by_revenue(
        self,
        publisher_id: str,
        limit: int = 3
    ) -> list[str]:
        """Get anchor bidders for publisher."""
        bidders = self._anchors.get(publisher_id, self._default)
        return bidders[:limit]
