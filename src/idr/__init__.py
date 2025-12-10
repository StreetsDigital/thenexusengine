"""
Intelligent Demand Router (IDR) for The Nexus Engine.

The IDR dynamically routes bid requests to demand partners based on
real-time performance data, historical analysis, and predictive scoring.
"""

from .classifier import RequestClassifier
from .models import (
    BidderMetrics,
    BidderScore,
    ClassifiedRequest,
    LookupKey,
    RecentMetrics,
    ScoreComponents,
)
from .scorer import BidderScorer
from .selector import PartnerSelector, SelectorConfig, SelectionResult

__version__ = '1.0.0'

__all__ = [
    'RequestClassifier',
    'BidderScorer',
    'PartnerSelector',
    'SelectorConfig',
    'SelectionResult',
    'ClassifiedRequest',
    'BidderScore',
    'ScoreComponents',
    'RecentMetrics',
    'BidderMetrics',
    'LookupKey',
]
