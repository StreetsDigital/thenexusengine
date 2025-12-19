"""
IDR Admin Dashboard - Simple web UI for managing IDR configuration.

Run with: python -m src.idr.admin.app

Authentication:
    Set admin credentials via environment variables:
    - ADMIN_USER_1=username:password
    - ADMIN_USER_2=username:password
    - ADMIN_USER_3=username:password

    Or use ADMIN_USERS for all at once:
    - ADMIN_USERS=user1:pass1,user2:pass2,user3:pass3
"""

import hashlib
import hmac
import os
import re
import secrets
import time
from datetime import datetime, timedelta
from functools import wraps
from pathlib import Path
from typing import Any, Optional

import yaml

try:
    from flask import Flask, jsonify, render_template, request, redirect, url_for, session, g
except ImportError:
    print("Flask not installed. Run: pip install flask")
    raise

# Import IDR components
try:
    from src.idr.classifier.request_classifier import RequestClassifier
    from src.idr.scorer.bidder_scorer import BidderScorer
    from src.idr.selector.partner_selector import PartnerSelector, SelectorConfig
    IDR_AVAILABLE = True
except ImportError:
    IDR_AVAILABLE = False

# Import database components
try:
    from src.idr.database.metrics_store import MetricsStore
    from src.idr.database.event_pipeline import EventPipeline, SyncEventPipeline
    DB_AVAILABLE = True
except ImportError:
    DB_AVAILABLE = False

# Global metrics store and event pipeline (initialized on first request)
_metrics_store = None
_event_pipeline = None


# =============================================================================
# Authentication System
# =============================================================================

def _hash_password(password: str, salt: str) -> str:
    """Hash a password with salt using PBKDF2."""
    return hashlib.pbkdf2_hmac(
        'sha256',
        password.encode('utf-8'),
        salt.encode('utf-8'),
        100000  # iterations
    ).hex()


def _parse_admin_users() -> dict[str, dict[str, str]]:
    """
    Parse admin users from environment variables.

    Supports two formats:
    1. Individual: ADMIN_USER_1=user:pass, ADMIN_USER_2=user:pass, etc.
    2. Combined: ADMIN_USERS=user1:pass1,user2:pass2,user3:pass3
    """
    users = {}
    salt = os.environ.get('ADMIN_SALT', 'nexus-engine-default-salt')

    # Try combined format first
    combined = os.environ.get('ADMIN_USERS', '')
    if combined:
        for pair in combined.split(','):
            pair = pair.strip()
            if ':' in pair:
                username, password = pair.split(':', 1)
                username = username.strip()
                password = password.strip()
                if username and password:
                    users[username] = {
                        'password_hash': _hash_password(password, salt),
                        'salt': salt
                    }

    # Also check individual user env vars (ADMIN_USER_1, ADMIN_USER_2, ADMIN_USER_3)
    for i in range(1, 4):
        user_env = os.environ.get(f'ADMIN_USER_{i}', '')
        if user_env and ':' in user_env:
            username, password = user_env.split(':', 1)
            username = username.strip()
            password = password.strip()
            if username and password:
                users[username] = {
                    'password_hash': _hash_password(password, salt),
                    'salt': salt
                }

    # If no users configured, create a default admin (with warning)
    if not users:
        default_pass = os.environ.get('ADMIN_DEFAULT_PASSWORD', '')
        if default_pass:
            users['admin'] = {
                'password_hash': _hash_password(default_pass, salt),
                'salt': salt
            }

    return users


def _verify_password(username: str, password: str, users: dict) -> bool:
    """Verify a password against stored hash."""
    if username not in users:
        return False

    user_data = users[username]
    expected_hash = user_data['password_hash']
    salt = user_data['salt']
    actual_hash = _hash_password(password, salt)

    # Use constant-time comparison to prevent timing attacks
    return hmac.compare_digest(expected_hash, actual_hash)


def _sanitize_publisher_id(publisher_id: str) -> str:
    """
    Sanitize publisher_id to prevent path traversal attacks.
    Only allows alphanumeric characters, hyphens, and underscores.
    """
    if not publisher_id:
        return ''
    # Remove any path separators and only allow safe characters
    sanitized = re.sub(r'[^a-zA-Z0-9_-]', '', publisher_id)
    # Ensure it doesn't start with a dash (could be interpreted as option)
    if sanitized.startswith('-'):
        sanitized = sanitized[1:]
    return sanitized[:64]  # Limit length


def _safe_error_response(error: Exception, generic_message: str, status_code: int = 500):
    """
    Return a safe error response without leaking internal details.
    Logs the actual error server-side for debugging.
    """
    # Log the actual error for debugging (server-side only)
    import logging
    logging.error(f"{generic_message}: {error}", exc_info=True)

    # Return generic message to client (no internal details)
    return {'status': 'error', 'message': generic_message}, status_code


# Default config path
# Path: src/idr/admin/app.py -> need 4 parents to reach thenexusengine root
DEFAULT_CONFIG_PATH = Path(__file__).parent.parent.parent.parent / "config" / "idr_config.yaml"


def load_config(config_path: Optional[Path] = None) -> dict[str, Any]:
    """Load configuration from YAML file."""
    path = config_path or DEFAULT_CONFIG_PATH
    if path.exists():
        with open(path) as f:
            return yaml.safe_load(f)
    return get_default_config()


def save_config(config: dict[str, Any], config_path: Optional[Path] = None) -> None:
    """Save configuration to YAML file."""
    path = config_path or DEFAULT_CONFIG_PATH
    path.parent.mkdir(parents=True, exist_ok=True)

    # Add header comment
    header = """# IDR Configuration
# Intelligent Demand Router settings for The Nexus Engine
# Last updated: {timestamp}

""".format(timestamp=datetime.now().isoformat())

    with open(path, 'w') as f:
        f.write(header)
        yaml.dump(config, f, default_flow_style=False, sort_keys=False)


def get_default_config() -> dict[str, Any]:
    """Get default configuration."""
    return {
        'scoring': {
            'weights': {
                'win_rate': 0.25,
                'bid_rate': 0.20,
                'cpm': 0.15,
                'floor_clearance': 0.15,
                'latency': 0.10,
                'recency': 0.10,
                'id_match': 0.05,
            }
        },
        'selector': {
            # Default to bypass (IDR off) during early development
            'bypass_enabled': True,
            'shadow_mode': False,
            'max_bidders': 15,
            'min_score_threshold': 25,
            'exploration_rate': 0.1,
            'exploration_slots': 2,
            'anchor_bidder_count': 3,
            'diversity_enabled': True,
            'diversity_categories': ['premium', 'mid_tier', 'video_specialist', 'native'],
        },
        'performance': {
            'min_sample_size': 100,
            'cold_start_threshold': 10000,
        },
        'latency': {
            'excellent_ms': 100,
            'poor_ms': 500,
        },
        'database': {
            'redis_url': os.environ.get('REDIS_URL', 'redis://localhost:6379'),
            'timescale_url': os.environ.get('TIMESCALE_URL', 'postgresql://postgres:postgres@localhost:5432/idr'),
            'event_buffer_size': 100,
            'flush_interval': 1,
            'use_mock': os.environ.get('USE_MOCK_DB', 'true').lower() == 'true',
        },
        'privacy': {
            'enabled': True,
            'strict_mode': False,
            'gdpr_enabled': True,
            'ccpa_enabled': True,
            'coppa_enabled': True,
        },
        'fpd': {
            'enabled': True,
            'site_enabled': True,
            'user_enabled': True,
            'imp_enabled': True,
            'global_enabled': False,
            'bidderconfig_enabled': False,
            'content_enabled': True,
            'eids_enabled': True,
            'eid_sources': 'liveramp.com,uidapi.com,id5-sync.com,criteo.com',
        },
    }


def create_app(config_path: Optional[Path] = None) -> Flask:
    """Create and configure the Flask application."""
    app = Flask(
        __name__,
        template_folder=str(Path(__file__).parent / 'templates'),
        static_folder=str(Path(__file__).parent / 'static'),
    )

    app.config['CONFIG_PATH'] = config_path or DEFAULT_CONFIG_PATH

    # Security configuration
    app.secret_key = os.environ.get('SECRET_KEY', secrets.token_hex(32))
    app.config['SESSION_COOKIE_SECURE'] = os.environ.get('SESSION_COOKIE_SECURE', 'false').lower() == 'true'
    app.config['SESSION_COOKIE_HTTPONLY'] = True
    app.config['SESSION_COOKIE_SAMESITE'] = 'Lax'
    app.config['PERMANENT_SESSION_LIFETIME'] = timedelta(hours=8)

    # Load admin users
    admin_users = _parse_admin_users()
    auth_enabled = bool(admin_users)

    if not auth_enabled:
        print("\n" + "=" * 60)
        print("  WARNING: No admin users configured!")
        print("  The admin dashboard is UNPROTECTED.")
        print("  Set ADMIN_USERS or ADMIN_USER_1/2/3 environment variables.")
        print("  Example: ADMIN_USERS=admin:secretpass123")
        print("=" * 60 + "\n")

    def login_required(f):
        """Decorator to require authentication for a route."""
        @wraps(f)
        def decorated_function(*args, **kwargs):
            if not auth_enabled:
                # Auth disabled - allow access but set a warning
                g.user = None
                return f(*args, **kwargs)

            if 'user' not in session:
                if request.is_json:
                    return jsonify({'error': 'Authentication required'}), 401
                return redirect(url_for('login'))

            g.user = session['user']
            return f(*args, **kwargs)
        return decorated_function

    # =================================
    # Authentication Routes
    # =================================

    @app.route('/login', methods=['GET', 'POST'])
    def login():
        """Login page and handler."""
        if not auth_enabled:
            return redirect(url_for('index'))

        error = None

        if request.method == 'POST':
            username = request.form.get('username', '').strip()
            password = request.form.get('password', '')

            if _verify_password(username, password, admin_users):
                session.permanent = True
                session['user'] = username
                session['login_time'] = datetime.now().isoformat()
                return redirect(url_for('index'))
            else:
                error = 'Invalid username or password'
                # Add small delay to prevent brute force
                time.sleep(0.5)

        return render_template('login.html', error=error)

    @app.route('/logout')
    def logout():
        """Logout handler."""
        session.clear()
        return redirect(url_for('login'))

    @app.route('/api/auth/status', methods=['GET'])
    def auth_status():
        """Check authentication status."""
        if not auth_enabled:
            return jsonify({
                'authenticated': True,
                'auth_enabled': False,
                'user': None,
                'warning': 'Authentication is disabled'
            })

        if 'user' in session:
            return jsonify({
                'authenticated': True,
                'auth_enabled': True,
                'user': session['user'],
                'login_time': session.get('login_time')
            })

        return jsonify({
            'authenticated': False,
            'auth_enabled': True,
            'user': None
        }), 401

    # =================================
    # Protected Routes
    # =================================

    @app.route('/')
    @login_required
    def index():
        """Main dashboard page."""
        config = load_config(app.config['CONFIG_PATH'])
        return render_template('index.html', config=config, user=getattr(g, 'user', None), auth_enabled=auth_enabled)

    @app.route('/bidders')
    @login_required
    def bidders_page():
        """OpenRTB Bidders management page."""
        return render_template('bidders.html', user=getattr(g, 'user', None), auth_enabled=auth_enabled)

    @app.route('/api/config', methods=['GET'])
    @login_required
    def get_config():
        """Get current configuration."""
        config = load_config(app.config['CONFIG_PATH'])
        return jsonify(config)

    @app.route('/api/config', methods=['POST'])
    @login_required
    def update_config():
        """Update configuration."""
        try:
            new_config = request.json
            save_config(new_config, app.config['CONFIG_PATH'])
            return jsonify({'status': 'success', 'message': 'Configuration saved'})
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to save configuration', 400))

    @app.route('/api/config/selector', methods=['PATCH'])
    @login_required
    def update_selector():
        """Update selector settings only."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            updates = request.json
            config['selector'].update(updates)
            save_config(config, app.config['CONFIG_PATH'])
            return jsonify({'status': 'success', 'config': config['selector']})
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to update selector settings', 400))

    @app.route('/api/config/scoring', methods=['PATCH'])
    @login_required
    def update_scoring():
        """Update scoring weights."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            weights = request.json.get('weights', {})

            # Validate weights sum to 1.0
            total = sum(weights.values())
            if abs(total - 1.0) > 0.01:
                return jsonify({
                    'status': 'error',
                    'message': f'Weights must sum to 1.0, got {total:.2f}'
                }), 400

            config['scoring']['weights'] = weights
            save_config(config, app.config['CONFIG_PATH'])
            return jsonify({'status': 'success', 'config': config['scoring']})
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to update scoring weights', 400))

    @app.route('/api/mode/bypass', methods=['POST'])
    @login_required
    def set_bypass_mode():
        """Quick toggle for bypass mode."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            enabled = request.json.get('enabled', False)
            config['selector']['bypass_enabled'] = enabled
            if enabled:
                config['selector']['shadow_mode'] = False  # Mutually exclusive
            save_config(config, app.config['CONFIG_PATH'])
            return jsonify({
                'status': 'success',
                'bypass_enabled': enabled,
                'message': 'Bypass mode ' + ('ENABLED - All bidders will be selected' if enabled else 'DISABLED')
            })
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to set bypass mode', 400))

    @app.route('/api/mode/shadow', methods=['POST'])
    @login_required
    def set_shadow_mode():
        """Quick toggle for shadow mode."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            enabled = request.json.get('enabled', False)
            config['selector']['shadow_mode'] = enabled
            if enabled:
                config['selector']['bypass_enabled'] = False  # Mutually exclusive
            save_config(config, app.config['CONFIG_PATH'])
            return jsonify({
                'status': 'success',
                'shadow_mode': enabled,
                'message': 'Shadow mode ' + ('ENABLED - Logging without filtering' if enabled else 'DISABLED')
            })
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to set shadow mode', 400))

    @app.route('/api/reset', methods=['POST'])
    @login_required
    def reset_config():
        """Reset to default configuration."""
        try:
            default = get_default_config()
            save_config(default, app.config['CONFIG_PATH'])
            return jsonify({'status': 'success', 'message': 'Configuration reset to defaults'})
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to reset configuration', 400))

    @app.route('/health', methods=['GET'])
    def health():
        """Health check endpoint for Go PBS server."""
        return jsonify({
            'status': 'healthy',
            'idr_available': IDR_AVAILABLE,
            'timestamp': datetime.now().isoformat()
        })

    @app.route('/api/status', methods=['GET'])
    @login_required
    def get_status():
        """Get infrastructure status (databases, pipeline)."""
        global _metrics_store, _event_pipeline

        status = {
            'redis': {'connected': False, 'error': 'Not initialized'},
            'timescale': {'connected': False, 'error': 'Not initialized'},
            'pipeline': {'active': False}
        }

        if _metrics_store is not None:
            # Check Redis
            try:
                redis_client = _metrics_store.redis
                if hasattr(redis_client, '_data'):  # Mock client
                    status['redis'] = {
                        'connected': False,
                        'error': 'Using mock client',
                        'keys': len(redis_client._data)
                    }
                else:
                    # Real Redis client would have ping
                    status['redis'] = {'connected': True, 'keys': 0}
            except Exception as e:
                status['redis'] = {'connected': False, 'error': str(e)}

            # Check TimescaleDB
            try:
                ts_client = _metrics_store.timescale
                if hasattr(ts_client, '_events'):  # Mock client
                    status['timescale'] = {
                        'connected': False,
                        'error': 'Using mock client',
                        'events': len(ts_client._events)
                    }
                else:
                    status['timescale'] = {'connected': True, 'events': 0}
            except Exception as e:
                status['timescale'] = {'connected': False, 'error': str(e)}

        if _event_pipeline is not None:
            try:
                if hasattr(_event_pipeline, '_buffer'):
                    status['pipeline'] = {
                        'active': True,
                        'buffered': len(_event_pipeline._buffer),
                        'processed': getattr(_event_pipeline, '_processed_count', 0)
                    }
                else:
                    status['pipeline'] = {'active': True, 'buffered': 0, 'processed': 0}
            except Exception:
                status['pipeline'] = {'active': False}

        return jsonify(status)

    @app.route('/api/config/database', methods=['PATCH'])
    @login_required
    def update_database_config():
        """Update database configuration."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            updates = request.json

            if 'database' not in config:
                config['database'] = {}

            config['database']['event_buffer_size'] = updates.get('event_buffer_size', 100)
            config['database']['flush_interval'] = updates.get('flush_interval', 1)
            config['database']['use_mock'] = updates.get('use_mock', False)

            save_config(config, app.config['CONFIG_PATH'])

            return jsonify({
                'status': 'success',
                'config': config['database'],
                'message': 'Database settings saved. Restart services to apply changes.'
            })
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to update database settings', 400))

    @app.route('/api/config/privacy', methods=['PATCH'])
    @login_required
    def update_privacy_config():
        """Update privacy compliance configuration."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            updates = request.json

            if 'privacy' not in config:
                config['privacy'] = {}

            config['privacy']['enabled'] = updates.get('enabled', True)
            config['privacy']['strict_mode'] = updates.get('strict_mode', False)

            # Also update selector config for consistency
            if 'selector' not in config:
                config['selector'] = {}
            config['selector']['privacy_enabled'] = config['privacy']['enabled']
            config['selector']['privacy_strict_mode'] = config['privacy']['strict_mode']

            save_config(config, app.config['CONFIG_PATH'])

            return jsonify({
                'status': 'success',
                'config': config['privacy'],
                'message': 'Privacy settings saved.'
            })
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to update privacy settings', 400))

    @app.route('/api/config/fpd', methods=['PATCH'])
    @login_required
    def update_fpd_config():
        """Update First Party Data (FPD) configuration."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            updates = request.json

            if 'fpd' not in config:
                config['fpd'] = {}

            config['fpd']['enabled'] = updates.get('enabled', True)
            config['fpd']['site_enabled'] = updates.get('site_enabled', True)
            config['fpd']['user_enabled'] = updates.get('user_enabled', True)
            config['fpd']['imp_enabled'] = updates.get('imp_enabled', True)
            config['fpd']['global_enabled'] = updates.get('global_enabled', False)
            config['fpd']['bidderconfig_enabled'] = updates.get('bidderconfig_enabled', False)
            config['fpd']['content_enabled'] = updates.get('content_enabled', True)
            config['fpd']['eids_enabled'] = updates.get('eids_enabled', True)
            config['fpd']['eid_sources'] = updates.get('eid_sources', '')

            save_config(config, app.config['CONFIG_PATH'])

            return jsonify({
                'status': 'success',
                'config': config['fpd'],
                'message': 'FPD settings saved.'
            })
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to update FPD settings', 400))

    @app.route('/api/config/cookie_sync', methods=['PATCH'])
    @login_required
    def update_cookie_sync_config():
        """Update Cookie Sync configuration."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            updates = request.json

            if 'cookie_sync' not in config:
                config['cookie_sync'] = {}

            config['cookie_sync']['enabled'] = updates.get('enabled', True)
            config['cookie_sync']['default_type'] = updates.get('default_type', 'iframe')
            config['cookie_sync']['limit'] = updates.get('limit', 5)
            config['cookie_sync']['interval_hours'] = updates.get('interval_hours', 24)
            config['cookie_sync']['sync_url'] = updates.get('sync_url', '/setuid')
            config['cookie_sync']['gdpr_url'] = updates.get('gdpr_url', '')
            config['cookie_sync']['coop_sync'] = updates.get('coop_sync', False)
            config['cookie_sync']['priority_sync'] = updates.get('priority_sync', True)

            save_config(config, app.config['CONFIG_PATH'])

            return jsonify({
                'status': 'success',
                'config': config['cookie_sync'],
                'message': 'Cookie sync settings saved.'
            })
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to update cookie sync settings', 400))

    @app.route('/api/select', methods=['POST'])
    @login_required
    def select_partners():
        """
        Select optimal bidding partners for an auction.

        Request body:
        {
            "request": { ... OpenRTB bid request ... },
            "available_bidders": ["appnexus", "rubicon", ...]
        }

        Response:
        {
            "selected_bidders": [
                {"bidder_code": "appnexus", "score": 85.5, "reason": "HIGH_SCORE"},
                ...
            ],
            "excluded_bidders": [...],  // Only in shadow mode
            "mode": "normal" | "shadow" | "bypass",
            "processing_time_ms": 12.5
        }
        """
        start_time = time.time()

        if not IDR_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'IDR components not available'
            }), 500

        try:
            data = request.json
            ortb_request = data.get('request', {})
            available_bidders = data.get('available_bidders', [])

            if not available_bidders:
                return jsonify({
                    'status': 'error',
                    'message': 'No available bidders provided'
                }), 400

            # Load current config
            config = load_config(app.config['CONFIG_PATH'])
            selector_config = config.get('selector', {})
            scoring_config = config.get('scoring', {})

            # Check for bypass mode
            if selector_config.get('bypass_enabled', False):
                return jsonify({
                    'selected_bidders': [
                        {'bidder_code': b, 'score': 0.0, 'reason': 'BYPASS'}
                        for b in available_bidders
                    ],
                    'excluded_bidders': [],
                    'mode': 'bypass',
                    'processing_time_ms': (time.time() - start_time) * 1000
                })

            # Initialize components
            classifier = RequestClassifier()
            scorer = BidderScorer(weights=scoring_config.get('weights'))

            sel_cfg = SelectorConfig(
                bypass_enabled=selector_config.get('bypass_enabled', False),
                shadow_mode=selector_config.get('shadow_mode', False),
                max_bidders=selector_config.get('max_bidders', 15),
                min_score_threshold=selector_config.get('min_score_threshold', 25),
                exploration_rate=selector_config.get('exploration_rate', 0.1),
                exploration_slots=selector_config.get('exploration_slots', 2),
                anchor_bidder_count=selector_config.get('anchor_bidder_count', 3),
                diversity_enabled=selector_config.get('diversity_enabled', True),
            )
            selector = PartnerSelector(config=sel_cfg)

            # Classify request
            classified = classifier.classify(ortb_request)

            # Score all available bidders
            scores = []
            for bidder in available_bidders:
                # In production, metrics would come from database
                score = scorer.score_bidder(bidder, classified)
                scores.append(score)

            # Select partners
            result = selector.select_partners(scores, classified)

            # Build response
            selected = [
                {
                    'bidder_code': s.bidder_code,
                    'score': s.final_score,
                    'reason': s.selection_reason.name if hasattr(s, 'selection_reason') else 'SELECTED'
                }
                for s in result.selected
            ]

            excluded = []
            if result.shadow_log:
                excluded = [
                    {
                        'bidder_code': e.bidder_code,
                        'score': e.final_score,
                        'reason': 'EXCLUDED'
                    }
                    for e in result.shadow_log
                ]

            mode = 'shadow' if sel_cfg.shadow_mode else 'normal'

            return jsonify({
                'selected_bidders': selected,
                'excluded_bidders': excluded,
                'mode': mode,
                'processing_time_ms': (time.time() - start_time) * 1000
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Partner selection failed', 500))

    @app.route('/api/events', methods=['POST'])
    @login_required
    def record_events():
        """
        Record auction events from PBS for metrics tracking.

        Request body:
        {
            "events": [
                {
                    "auction_id": "...",
                    "bidder_code": "appnexus",
                    "event_type": "bid_response",
                    "latency_ms": 150,
                    "had_bid": true,
                    "bid_cpm": 2.50,
                    "country": "US",
                    "device_type": "mobile",
                    "media_type": "banner",
                    "ad_size": "300x250"
                },
                ...
            ]
        }
        """
        global _metrics_store, _event_pipeline

        if not DB_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Database components not available'
            }), 500

        try:
            # Initialize metrics store if needed
            if _metrics_store is None:
                _metrics_store = MetricsStore.create(use_mocks=True)

            # Initialize event pipeline if needed
            if _event_pipeline is None:
                _event_pipeline = SyncEventPipeline(_metrics_store)

            data = request.json
            events = data.get('events', [])

            if not events:
                return jsonify({
                    'status': 'error',
                    'message': 'No events provided'
                }), 400

            processed = 0
            for event_data in events:
                event_type = event_data.get('event_type', 'bid_response')

                if event_type == 'win':
                    _event_pipeline.submit_win(
                        auction_id=event_data.get('auction_id', ''),
                        bidder_code=event_data.get('bidder_code', ''),
                        win_cpm=event_data.get('win_cpm', 0),
                        country=event_data.get('country', ''),
                        device_type=event_data.get('device_type', ''),
                        media_type=event_data.get('media_type', ''),
                        ad_size=event_data.get('ad_size', ''),
                        publisher_id=event_data.get('publisher_id', ''),
                    )
                else:
                    _event_pipeline.submit_bid_response(
                        auction_id=event_data.get('auction_id', ''),
                        bidder_code=event_data.get('bidder_code', ''),
                        had_bid=event_data.get('had_bid', False),
                        latency_ms=event_data.get('latency_ms', 0),
                        bid_cpm=event_data.get('bid_cpm'),
                        floor_price=event_data.get('floor_price'),
                        country=event_data.get('country', ''),
                        device_type=event_data.get('device_type', ''),
                        media_type=event_data.get('media_type', ''),
                        ad_size=event_data.get('ad_size', ''),
                        publisher_id=event_data.get('publisher_id', ''),
                        timed_out=event_data.get('timed_out', False),
                        error_message=event_data.get('error_message'),
                    )
                processed += 1

            return jsonify({
                'status': 'success',
                'processed': processed
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to record events', 500))

    @app.route('/api/metrics', methods=['GET'])
    @login_required
    def get_metrics():
        """Get current bidder metrics."""
        global _metrics_store

        if not DB_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Database components not available'
            }), 500

        try:
            if _metrics_store is None:
                _metrics_store = MetricsStore.create(use_mocks=True)

            all_metrics = _metrics_store.get_all_metrics()

            return jsonify({
                'bidders': {
                    code: {
                        'win_rate': m.win_rate,
                        'bid_rate': m.bid_rate,
                        'avg_cpm': m.avg_cpm,
                        'p95_latency_ms': m.p95_latency_ms,
                        'total_requests': m.total_requests,
                        'confidence': m.confidence,
                    }
                    for code, m in all_metrics.items()
                }
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to load metrics', 500))

    @app.route('/api/metrics/<bidder_code>', methods=['GET'])
    @login_required
    def get_bidder_metrics(bidder_code: str):
        """Get metrics for a specific bidder."""
        global _metrics_store

        if not DB_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Database components not available'
            }), 500

        try:
            if _metrics_store is None:
                _metrics_store = MetricsStore.create(use_mocks=True)

            m = _metrics_store.get_metrics(bidder_code)

            return jsonify({
                'bidder_code': bidder_code,
                'win_rate': m.win_rate,
                'bid_rate': m.bid_rate,
                'avg_cpm': m.avg_cpm,
                'floor_clearance_rate': m.floor_clearance_rate,
                'avg_latency_ms': m.avg_latency_ms,
                'p95_latency_ms': m.p95_latency_ms,
                'total_requests': m.total_requests,
                'realtime_requests': m.realtime_requests,
                'historical_requests': m.historical_requests,
                'timeout_rate': m.timeout_rate,
                'error_rate': m.error_rate,
                'confidence': m.confidence,
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to load bidder metrics', 500))

    # =========================================
    # Publisher Management Endpoints
    # =========================================

    # Import publisher config manager
    try:
        from src.idr.config.publisher_config import (
            PublisherConfigManager,
            get_publisher_config_manager,
        )
        PUBLISHER_CONFIG_AVAILABLE = True
    except ImportError:
        PUBLISHER_CONFIG_AVAILABLE = False

    @app.route('/api/publishers', methods=['GET'])
    @login_required
    def list_publishers():
        """List all configured publishers."""
        if not PUBLISHER_CONFIG_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Publisher config module not available'
            }), 500

        try:
            manager = get_publisher_config_manager()
            configs = manager.load_all()

            publishers = []
            for pub_id, config in configs.items():
                publishers.append({
                    'id': config.publisher_id,
                    'name': config.name,
                    'enabled': config.enabled,
                    'sites': len(config.sites),
                    'bidders': len(config.get_enabled_bidders()),
                    'contact_email': config.contact_email,
                })

            return jsonify({
                'publishers': publishers,
                'total': len(publishers)
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to list publishers', 500))

    @app.route('/api/publishers/<publisher_id>', methods=['GET'])
    @login_required
    def get_publisher(publisher_id: str):
        """Get configuration for a specific publisher."""
        if not PUBLISHER_CONFIG_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Publisher config module not available'
            }), 500

        # Sanitize publisher_id to prevent path traversal
        safe_publisher_id = _sanitize_publisher_id(publisher_id)
        if not safe_publisher_id or safe_publisher_id != publisher_id:
            return jsonify({
                'status': 'error',
                'message': 'Invalid publisher ID format'
            }), 400

        try:
            manager = get_publisher_config_manager()
            config = manager.get(safe_publisher_id)

            if config is None:
                return jsonify({
                    'status': 'error',
                    'message': f'Publisher {publisher_id} not found'
                }), 404

            return jsonify({
                'publisher_id': config.publisher_id,
                'name': config.name,
                'enabled': config.enabled,
                'contact': {
                    'email': config.contact_email,
                    'name': config.contact_name,
                },
                'sites': [
                    {'site_id': s.site_id, 'domain': s.domain, 'name': s.name}
                    for s in config.sites
                ],
                'bidders': {
                    code: {'enabled': bc.enabled, 'params': bc.params}
                    for code, bc in config.bidders.items()
                },
                'idr': {
                    'max_bidders': config.idr.max_bidders,
                    'min_score': config.idr.min_score,
                    'timeout_ms': config.idr.timeout_ms,
                },
                'rate_limits': {
                    'requests_per_second': config.rate_limits.requests_per_second,
                    'burst': config.rate_limits.burst,
                },
                'privacy': {
                    'gdpr_applies': config.privacy.gdpr_applies,
                    'ccpa_applies': config.privacy.ccpa_applies,
                    'coppa_applies': config.privacy.coppa_applies,
                },
                'revenue_share': {
                    'platform_demand_rev_share': config.revenue_share.platform_demand_rev_share,
                    'publisher_own_demand_fee': config.revenue_share.publisher_own_demand_fee,
                },
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to load publisher', 500))

    @app.route('/api/publishers/<publisher_id>', methods=['PUT'])
    @login_required
    def save_publisher(publisher_id: str):
        """Save/update a publisher configuration."""
        if not PUBLISHER_CONFIG_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Publisher config module not available'
            }), 500

        # Sanitize publisher_id to prevent path traversal
        safe_publisher_id = _sanitize_publisher_id(publisher_id)
        if not safe_publisher_id or safe_publisher_id != publisher_id:
            return jsonify({
                'status': 'error',
                'message': 'Invalid publisher ID format. Use only alphanumeric characters, hyphens, and underscores.'
            }), 400

        try:
            data = request.json

            # Build YAML content
            config_content = {
                'publisher_id': safe_publisher_id,
                'name': data.get('name', safe_publisher_id),
                'enabled': data.get('enabled', True),
                'contact': data.get('contact', {}),
                'sites': data.get('sites', []),
                'bidders': data.get('bidders', {}),
                'idr': data.get('idr', {'max_bidders': 8, 'min_score': 0.1, 'timeout_ms': 50}),
                'rate_limits': data.get('rate_limits', {'requests_per_second': 1000, 'burst': 100}),
                'privacy': data.get('privacy', {'gdpr_applies': True, 'ccpa_applies': True, 'coppa_applies': False}),
                'revenue_share': data.get('revenue_share', {'platform_demand_rev_share': 0.0, 'publisher_own_demand_fee': 0.0}),
            }

            # Get config directory
            manager = get_publisher_config_manager()
            config_path = manager.config_dir / f"{safe_publisher_id}.yaml"

            # Ensure directory exists
            config_path.parent.mkdir(parents=True, exist_ok=True)

            # Write config
            with open(config_path, 'w') as f:
                yaml.dump(config_content, f, default_flow_style=False, sort_keys=False)

            # Reload config
            manager.reload(safe_publisher_id)

            return jsonify({
                'status': 'success',
                'message': f'Publisher {safe_publisher_id} saved',
                'path': str(config_path)
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to save publisher', 500))

    @app.route('/api/publishers/<publisher_id>', methods=['DELETE'])
    @login_required
    def delete_publisher(publisher_id: str):
        """Delete a publisher configuration."""
        if not PUBLISHER_CONFIG_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Publisher config module not available'
            }), 500

        # Sanitize publisher_id to prevent path traversal
        safe_publisher_id = _sanitize_publisher_id(publisher_id)
        if not safe_publisher_id or safe_publisher_id != publisher_id:
            return jsonify({
                'status': 'error',
                'message': 'Invalid publisher ID format'
            }), 400

        try:
            manager = get_publisher_config_manager()
            config_path = manager.config_dir / f"{safe_publisher_id}.yaml"

            if not config_path.exists():
                return jsonify({
                    'status': 'error',
                    'message': f'Publisher {safe_publisher_id} not found'
                }), 404

            # Remove file
            config_path.unlink()

            # Clear from cache
            manager.reload(safe_publisher_id)

            return jsonify({
                'status': 'success',
                'message': f'Publisher {safe_publisher_id} deleted'
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to delete publisher', 500))

    @app.route('/api/publishers/reload', methods=['POST'])
    @login_required
    def reload_publishers():
        """Reload all publisher configurations from disk."""
        if not PUBLISHER_CONFIG_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Publisher config module not available'
            }), 500

        try:
            manager = get_publisher_config_manager()
            manager.reload()
            configs = manager.load_all()

            return jsonify({
                'status': 'success',
                'message': f'Reloaded {len(configs)} publisher configurations',
                'publishers': list(configs.keys())
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to reload publishers', 500))

    # Legacy endpoint for backwards compatibility
    @app.route('/admin/reload-configs', methods=['POST'])
    @login_required
    def reload_configs_legacy():
        """Legacy endpoint - redirects to new API."""
        return reload_publishers()

    # =========================================
    # Hierarchical Configuration API (v2)
    # =========================================

    try:
        from src.idr.config.config_api import create_config_api_blueprint
        config_api_bp = create_config_api_blueprint()
        app.register_blueprint(config_api_bp)
        CONFIG_API_AVAILABLE = True
    except ImportError:
        CONFIG_API_AVAILABLE = False

    @app.route('/api/v2/status', methods=['GET'])
    @login_required
    def config_api_status():
        """Check if the v2 configuration API is available."""
        return jsonify({
            'config_api_available': CONFIG_API_AVAILABLE,
            'version': '2.0',
            'features': [
                'hierarchical_configuration',
                'publisher_level_config',
                'site_level_config',
                'ad_unit_level_config',
                'config_inheritance',
                'bulk_operations',
            ] if CONFIG_API_AVAILABLE else [],
        })

    # =========================================
    # API Key Management Endpoints
    # =========================================

    # Import API key manager
    try:
        from src.idr.auth.api_keys import get_api_key_manager, APIKeyManager
        API_KEY_MANAGER_AVAILABLE = True
    except ImportError:
        API_KEY_MANAGER_AVAILABLE = False

    @app.route('/api/keys', methods=['GET'])
    @login_required
    def list_api_keys():
        """List all API keys."""
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'API key manager not available'
            }), 500

        try:
            manager = get_api_key_manager()
            keys = manager.list_keys()

            return jsonify({
                'keys': [
                    {
                        'key': k.masked_key,
                        'publisher_id': k.publisher_id,
                        'created_at': k.created_at,
                        'last_used': k.last_used,
                        'enabled': k.enabled,
                        'request_count': k.request_count,
                    }
                    for k in keys
                ],
                'total': len(keys)
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to list API keys', 500))

    @app.route('/api/keys', methods=['POST'])
    @login_required
    def generate_api_key():
        """Generate a new API key for a publisher."""
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'API key manager not available'
            }), 500

        try:
            data = request.json
            publisher_id = data.get('publisher_id')
            environment = data.get('environment', 'live')
            replace_existing = data.get('replace_existing', False)

            if not publisher_id:
                return jsonify({
                    'status': 'error',
                    'message': 'publisher_id is required'
                }), 400

            # Sanitize publisher_id
            safe_publisher_id = _sanitize_publisher_id(publisher_id)
            if not safe_publisher_id or safe_publisher_id != publisher_id:
                return jsonify({
                    'status': 'error',
                    'message': 'Invalid publisher ID format'
                }), 400

            manager = get_api_key_manager()
            api_key = manager.generate_key(
                safe_publisher_id,
                environment=environment,
                replace_existing=replace_existing
            )

            if not api_key:
                # Key might already exist
                existing = manager.get_publisher_key(safe_publisher_id)
                if existing and not replace_existing:
                    return jsonify({
                        'status': 'error',
                        'message': 'API key already exists for this publisher. Set replace_existing=true to regenerate.'
                    }), 409

                return jsonify({
                    'status': 'error',
                    'message': 'Failed to generate API key. Check Redis connection.'
                }), 500

            return jsonify({
                'status': 'success',
                'api_key': api_key,
                'publisher_id': safe_publisher_id,
                'environment': environment,
                'message': 'API key generated successfully. Store this key securely - it cannot be retrieved again.'
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to generate API key', 500))

    @app.route('/api/keys/<api_key>', methods=['DELETE'])
    @login_required
    def revoke_api_key(api_key: str):
        """Revoke an API key."""
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'API key manager not available'
            }), 500

        try:
            manager = get_api_key_manager()
            success = manager.revoke_key(api_key)

            if not success:
                return jsonify({
                    'status': 'error',
                    'message': 'API key not found'
                }), 404

            return jsonify({
                'status': 'success',
                'message': 'API key revoked'
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to revoke API key', 500))

    @app.route('/api/keys/<api_key>/disable', methods=['POST'])
    @login_required
    def disable_api_key(api_key: str):
        """Disable an API key without revoking it."""
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'API key manager not available'
            }), 500

        try:
            manager = get_api_key_manager()
            success = manager.disable_key(api_key)

            if not success:
                return jsonify({
                    'status': 'error',
                    'message': 'API key not found'
                }), 404

            return jsonify({
                'status': 'success',
                'message': 'API key disabled'
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to disable API key', 500))

    @app.route('/api/keys/<api_key>/enable', methods=['POST'])
    @login_required
    def enable_api_key(api_key: str):
        """Re-enable a disabled API key."""
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'API key manager not available'
            }), 500

        try:
            manager = get_api_key_manager()
            success = manager.enable_key(api_key)

            if not success:
                return jsonify({
                    'status': 'error',
                    'message': 'API key not found'
                }), 404

            return jsonify({
                'status': 'success',
                'message': 'API key enabled'
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to enable API key', 500))

    @app.route('/api/publishers/<publisher_id>/key', methods=['GET'])
    @login_required
    def get_publisher_api_key(publisher_id: str):
        """Get the API key info for a publisher (masked)."""
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'API key manager not available'
            }), 500

        # Sanitize publisher_id
        safe_publisher_id = _sanitize_publisher_id(publisher_id)
        if not safe_publisher_id or safe_publisher_id != publisher_id:
            return jsonify({
                'status': 'error',
                'message': 'Invalid publisher ID format'
            }), 400

        try:
            manager = get_api_key_manager()
            api_key = manager.get_publisher_key(safe_publisher_id)

            if not api_key:
                return jsonify({
                    'status': 'error',
                    'message': 'No API key found for this publisher'
                }), 404

            info = manager.get_key_info(api_key)
            if not info:
                return jsonify({
                    'status': 'error',
                    'message': 'API key metadata not found'
                }), 404

            return jsonify({
                'publisher_id': safe_publisher_id,
                'key': info.masked_key,
                'created_at': info.created_at,
                'last_used': info.last_used,
                'enabled': info.enabled,
                'request_count': info.request_count,
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to get publisher API key', 500))

    @app.route('/api/publishers/<publisher_id>/key', methods=['POST'])
    @login_required
    def create_publisher_api_key(publisher_id: str):
        """Create or regenerate API key for a publisher."""
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'API key manager not available'
            }), 500

        # Sanitize publisher_id
        safe_publisher_id = _sanitize_publisher_id(publisher_id)
        if not safe_publisher_id or safe_publisher_id != publisher_id:
            return jsonify({
                'status': 'error',
                'message': 'Invalid publisher ID format'
            }), 400

        try:
            data = request.json or {}
            environment = data.get('environment', 'live')
            regenerate = data.get('regenerate', False)

            manager = get_api_key_manager()
            api_key = manager.generate_key(
                safe_publisher_id,
                environment=environment,
                replace_existing=regenerate
            )

            if not api_key:
                existing = manager.get_publisher_key(safe_publisher_id)
                if existing:
                    return jsonify({
                        'status': 'error',
                        'message': 'API key already exists. Set regenerate=true to create a new one.'
                    }), 409
                return jsonify({
                    'status': 'error',
                    'message': 'Failed to generate API key'
                }), 500

            return jsonify({
                'status': 'success',
                'api_key': api_key,
                'publisher_id': safe_publisher_id,
                'message': 'Store this key securely - it cannot be retrieved again.'
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to create API key', 500))

    # =========================================
    # API Key Validation Endpoint (for PBS)
    # =========================================

    @app.route('/api/validate-key', methods=['POST'])
    def validate_api_key():
        """
        Validate an API key (called by PBS).

        This endpoint does NOT require admin authentication since
        it's called by PBS on every request.
        """
        if not API_KEY_MANAGER_AVAILABLE:
            return jsonify({
                'valid': False,
                'error': 'API key manager not available'
            }), 500

        try:
            data = request.json
            api_key = data.get('api_key', '')

            if not api_key:
                return jsonify({
                    'valid': False,
                    'error': 'api_key is required'
                }), 400

            manager = get_api_key_manager()
            publisher_id = manager.validate_key(api_key)

            if publisher_id:
                return jsonify({
                    'valid': True,
                    'publisher_id': publisher_id
                })
            else:
                return jsonify({
                    'valid': False,
                    'error': 'Invalid API key'
                }), 401

        except Exception as e:
            return jsonify({
                'valid': False,
                'error': 'Validation failed'
            }), 500

    # =========================================
    # OpenRTB Bidder Management Endpoints
    # =========================================

    # Import bidder manager
    try:
        from src.idr.bidders import (
            BidderManager,
            get_bidder_manager,
            BidderConfig,
            BidderStatus,
            BidderNotFoundError,
            BidderAlreadyExistsError,
            InvalidBidderConfigError,
        )
        BIDDER_MANAGER_AVAILABLE = True
    except ImportError:
        BIDDER_MANAGER_AVAILABLE = False

    def _sanitize_bidder_code(bidder_code: str) -> str:
        """Sanitize bidder code to prevent injection attacks."""
        if not bidder_code:
            return ''
        # Only allow lowercase alphanumeric and hyphens
        sanitized = re.sub(r'[^a-z0-9-]', '', bidder_code.lower())
        return sanitized[:64]

    @app.route('/api/bidders', methods=['GET'])
    @login_required
    def list_bidders():
        """
        List all configured OpenRTB bidders.

        Query parameters:
            include_disabled: Include disabled bidders (default: true)
            publisher_id: Filter by publisher access (optional)
        """
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        try:
            manager = get_bidder_manager()
            include_disabled = request.args.get('include_disabled', 'true').lower() == 'true'
            publisher_id = request.args.get('publisher_id')

            if publisher_id:
                safe_publisher_id = _sanitize_publisher_id(publisher_id)
                bidders = manager.get_bidders_for_publisher(safe_publisher_id)
            else:
                bidders = manager.list_bidders(include_disabled=include_disabled)

            return jsonify({
                'bidders': [
                    {
                        'bidder_code': b.bidder_code,
                        'name': b.name,
                        'description': b.description,
                        'endpoint_url': b.endpoint.url,
                        'status': b.status.value,
                        'media_types': b.capabilities.media_types,
                        'priority': b.priority,
                        'gvl_vendor_id': b.gvl_vendor_id,
                        'created_at': b.created_at,
                        'updated_at': b.updated_at,
                        'stats': {
                            'total_requests': b.total_requests,
                            'total_bids': b.total_bids,
                            'total_wins': b.total_wins,
                            'bid_rate': b.bid_rate,
                            'win_rate': b.win_rate,
                            'avg_latency_ms': b.avg_latency_ms,
                            'avg_bid_cpm': b.avg_bid_cpm,
                        },
                    }
                    for b in bidders
                ],
                'total': len(bidders)
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to list bidders', 500))

    @app.route('/api/bidders', methods=['POST'])
    @login_required
    def create_bidder():
        """
        Create a new OpenRTB bidder.

        Request body:
        {
            "name": "My DSP",
            "endpoint_url": "https://dsp.example.com/bid",
            "media_types": ["banner", "video"],
            "bidder_code": "my-dsp",  // Optional, generated from name
            "description": "My demand source",
            "timeout_ms": 200,
            "protocol_version": "2.6",
            "auth_type": "bearer",
            "auth_token": "secret",
            "gvl_vendor_id": 123,
            "priority": 50,
            "capabilities": {...},
            "request_transform": {...},
            "response_transform": {...}
        }
        """
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        try:
            data = request.json

            # Validate required fields
            if not data.get('name'):
                return jsonify({
                    'status': 'error',
                    'message': 'name is required'
                }), 400

            if not data.get('endpoint_url'):
                return jsonify({
                    'status': 'error',
                    'message': 'endpoint_url is required'
                }), 400

            manager = get_bidder_manager()

            # Extract parameters
            kwargs = {}
            if 'maintainer_email' in data:
                kwargs['maintainer_email'] = data['maintainer_email']
            if 'maintainer_name' in data:
                kwargs['maintainer_name'] = data['maintainer_name']
            if 'allowed_publishers' in data:
                kwargs['allowed_publishers'] = data['allowed_publishers']
            if 'blocked_publishers' in data:
                kwargs['blocked_publishers'] = data['blocked_publishers']
            if 'allowed_countries' in data:
                kwargs['allowed_countries'] = data['allowed_countries']
            if 'blocked_countries' in data:
                kwargs['blocked_countries'] = data['blocked_countries']

            bidder = manager.create_bidder(
                name=data['name'],
                endpoint_url=data['endpoint_url'],
                media_types=data.get('media_types', ['banner']),
                bidder_code=data.get('bidder_code'),
                description=data.get('description', ''),
                timeout_ms=data.get('timeout_ms', 200),
                protocol_version=data.get('protocol_version', '2.6'),
                auth_type=data.get('auth_type'),
                auth_token=data.get('auth_token'),
                gvl_vendor_id=data.get('gvl_vendor_id'),
                priority=data.get('priority', 50),
                custom_headers=data.get('custom_headers'),
                **kwargs
            )

            return jsonify({
                'status': 'success',
                'bidder_code': bidder.bidder_code,
                'message': f'Bidder {bidder.name} created successfully',
                'bidder': bidder.to_dict()
            }), 201

        except BidderAlreadyExistsError as e:
            return jsonify({
                'status': 'error',
                'message': str(e)
            }), 409
        except InvalidBidderConfigError as e:
            return jsonify({
                'status': 'error',
                'message': str(e)
            }), 400
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to create bidder', 500))

    @app.route('/api/bidders/<bidder_code>', methods=['GET'])
    @login_required
    def get_bidder(bidder_code: str):
        """Get a specific bidder configuration."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code or safe_code != bidder_code.lower():
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code format'
            }), 400

        try:
            manager = get_bidder_manager()
            bidder = manager.get_bidder(safe_code)
            stats = manager.get_stats(safe_code)

            response_data = bidder.to_dict()
            response_data['realtime_stats'] = stats

            return jsonify(response_data)

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Bidder not found: {safe_code}'
            }), 404
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to get bidder', 500))

    @app.route('/api/bidders/<bidder_code>', methods=['PUT', 'PATCH'])
    @login_required
    def update_bidder(bidder_code: str):
        """
        Update a bidder configuration.

        PUT replaces the entire config, PATCH updates specific fields.
        """
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code or safe_code != bidder_code.lower():
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code format'
            }), 400

        try:
            manager = get_bidder_manager()
            data = request.json

            # Map nested fields for endpoint updates
            if 'endpoint_url' in data:
                data['endpoint_url'] = data['endpoint_url']
            if 'endpoint' in data:
                endpoint = data.pop('endpoint')
                for k, v in endpoint.items():
                    data[f'endpoint_{k}'] = v

            # Map nested fields for capabilities
            if 'capabilities' in data:
                caps = data.pop('capabilities')
                for k, v in caps.items():
                    data[f'capabilities_{k}'] = v

            bidder = manager.update_bidder(safe_code, **data)

            return jsonify({
                'status': 'success',
                'message': f'Bidder {safe_code} updated',
                'bidder': bidder.to_dict()
            })

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Bidder not found: {safe_code}'
            }), 404
        except InvalidBidderConfigError as e:
            return jsonify({
                'status': 'error',
                'message': str(e)
            }), 400
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to update bidder', 500))

    @app.route('/api/bidders/<bidder_code>', methods=['DELETE'])
    @login_required
    def delete_bidder(bidder_code: str):
        """Delete a bidder."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code or safe_code != bidder_code.lower():
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code format'
            }), 400

        try:
            manager = get_bidder_manager()
            manager.delete_bidder(safe_code)

            return jsonify({
                'status': 'success',
                'message': f'Bidder {safe_code} deleted'
            })

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Bidder not found: {safe_code}'
            }), 404
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to delete bidder', 500))

    @app.route('/api/bidders/<bidder_code>/enable', methods=['POST'])
    @login_required
    def enable_bidder(bidder_code: str):
        """Enable a bidder (set status to active)."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code:
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code'
            }), 400

        try:
            manager = get_bidder_manager()
            bidder = manager.enable_bidder(safe_code)

            return jsonify({
                'status': 'success',
                'message': f'Bidder {safe_code} enabled',
                'bidder_status': bidder.status.value
            })

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Bidder not found: {safe_code}'
            }), 404
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to enable bidder', 500))

    @app.route('/api/bidders/<bidder_code>/disable', methods=['POST'])
    @login_required
    def disable_bidder(bidder_code: str):
        """Disable a bidder."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code:
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code'
            }), 400

        try:
            manager = get_bidder_manager()
            bidder = manager.disable_bidder(safe_code)

            return jsonify({
                'status': 'success',
                'message': f'Bidder {safe_code} disabled',
                'bidder_status': bidder.status.value
            })

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Bidder not found: {safe_code}'
            }), 404
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to disable bidder', 500))

    @app.route('/api/bidders/<bidder_code>/pause', methods=['POST'])
    @login_required
    def pause_bidder(bidder_code: str):
        """Pause a bidder temporarily."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code:
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code'
            }), 400

        try:
            manager = get_bidder_manager()
            bidder = manager.pause_bidder(safe_code)

            return jsonify({
                'status': 'success',
                'message': f'Bidder {safe_code} paused',
                'bidder_status': bidder.status.value
            })

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Bidder not found: {safe_code}'
            }), 404
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to pause bidder', 500))

    @app.route('/api/bidders/<bidder_code>/test', methods=['POST'])
    @login_required
    def test_bidder(bidder_code: str):
        """
        Test a bidder's endpoint with a sample OpenRTB request.

        Returns connection status, latency, and sample response.
        """
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code:
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code'
            }), 400

        try:
            manager = get_bidder_manager()
            result = manager.test_endpoint(safe_code)

            return jsonify({
                'status': 'success' if result.get('success') else 'error',
                'test_result': result
            })

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Bidder not found: {safe_code}'
            }), 404
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to test bidder', 500))

    @app.route('/api/bidders/<bidder_code>/stats', methods=['GET'])
    @login_required
    def get_bidder_stats(bidder_code: str):
        """Get real-time statistics for a bidder."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code:
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code'
            }), 400

        try:
            manager = get_bidder_manager()
            stats = manager.get_stats(safe_code)

            if not stats:
                return jsonify({
                    'bidder_code': safe_code,
                    'message': 'No statistics available yet',
                    'stats': {}
                })

            return jsonify({
                'bidder_code': safe_code,
                'stats': stats
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to get bidder stats', 500))

    @app.route('/api/bidders/<bidder_code>/stats/reset', methods=['POST'])
    @login_required
    def reset_bidder_stats(bidder_code: str):
        """Reset statistics for a bidder."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code:
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code'
            }), 400

        try:
            manager = get_bidder_manager()
            manager.reset_stats(safe_code)

            return jsonify({
                'status': 'success',
                'message': f'Statistics reset for {safe_code}'
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to reset bidder stats', 500))

    @app.route('/api/bidders/<bidder_code>/duplicate', methods=['POST'])
    @login_required
    def duplicate_bidder(bidder_code: str):
        """
        Create a copy of an existing bidder.

        Request body:
        {
            "new_name": "My DSP Copy",
            "new_endpoint_url": "https://new-endpoint.example.com/bid"  // Optional
        }
        """
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        safe_code = _sanitize_bidder_code(bidder_code)
        if not safe_code:
            return jsonify({
                'status': 'error',
                'message': 'Invalid bidder code'
            }), 400

        try:
            data = request.json
            new_name = data.get('new_name')

            if not new_name:
                return jsonify({
                    'status': 'error',
                    'message': 'new_name is required'
                }), 400

            manager = get_bidder_manager()
            new_bidder = manager.duplicate_bidder(
                source_code=safe_code,
                new_name=new_name,
                new_endpoint_url=data.get('new_endpoint_url')
            )

            return jsonify({
                'status': 'success',
                'message': f'Bidder duplicated as {new_bidder.bidder_code}',
                'bidder': new_bidder.to_dict()
            }), 201

        except BidderNotFoundError:
            return jsonify({
                'status': 'error',
                'message': f'Source bidder not found: {safe_code}'
            }), 404
        except BidderAlreadyExistsError as e:
            return jsonify({
                'status': 'error',
                'message': str(e)
            }), 409
        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to duplicate bidder', 500))

    @app.route('/api/bidders/export', methods=['GET'])
    @login_required
    def export_bidders():
        """Export all bidder configurations as JSON."""
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        try:
            manager = get_bidder_manager()
            bidders = manager.list_bidders(include_disabled=True)

            return jsonify({
                'bidders': [b.to_dict() for b in bidders],
                'exported_at': datetime.now().isoformat(),
                'count': len(bidders)
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to export bidders', 500))

    @app.route('/api/bidders/import', methods=['POST'])
    @login_required
    def import_bidders():
        """
        Import bidder configurations from JSON.

        Request body:
        {
            "bidders": [{...}, {...}],
            "overwrite": false  // If true, overwrite existing bidders
        }
        """
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'status': 'error',
                'message': 'Bidder manager not available'
            }), 500

        try:
            data = request.json
            bidders_data = data.get('bidders', [])
            overwrite = data.get('overwrite', False)

            if not bidders_data:
                return jsonify({
                    'status': 'error',
                    'message': 'No bidders to import'
                }), 400

            manager = get_bidder_manager()
            imported = []
            skipped = []
            errors = []

            for bidder_dict in bidders_data:
                try:
                    code = bidder_dict.get('bidder_code', '')

                    # Check if exists
                    try:
                        existing = manager.get_bidder(code)
                        if not overwrite:
                            skipped.append(code)
                            continue
                        # Delete existing if overwrite
                        manager.delete_bidder(code)
                    except BidderNotFoundError:
                        pass

                    # Import
                    bidder = manager.import_bidder(bidder_dict)
                    imported.append(bidder.bidder_code)

                except Exception as e:
                    errors.append({
                        'bidder_code': bidder_dict.get('bidder_code', 'unknown'),
                        'error': str(e)
                    })

            return jsonify({
                'status': 'success',
                'imported': imported,
                'skipped': skipped,
                'errors': errors,
                'summary': {
                    'total': len(bidders_data),
                    'imported': len(imported),
                    'skipped': len(skipped),
                    'failed': len(errors)
                }
            })

        except Exception as e:
            return jsonify(_safe_error_response(e, 'Failed to import bidders', 500))

    # =========================================
    # Bidder Listing for PBS (No Auth Required)
    # =========================================

    @app.route('/api/bidders/active', methods=['GET'])
    def list_active_bidders():
        """
        List active bidders for PBS.

        This endpoint does NOT require admin authentication since
        it's called by PBS to get available bidders.
        """
        if not BIDDER_MANAGER_AVAILABLE:
            return jsonify({
                'bidders': [],
                'error': 'Bidder manager not available'
            }), 500

        try:
            manager = get_bidder_manager()
            publisher_id = request.args.get('publisher_id')
            country = request.args.get('country')

            if publisher_id:
                safe_pub_id = _sanitize_publisher_id(publisher_id)
                bidders = manager.get_bidders_for_publisher(safe_pub_id, country)
            else:
                bidders = manager.get_active_bidders()

            return jsonify({
                'bidders': [
                    {
                        'bidder_code': b.bidder_code,
                        'endpoint': b.endpoint.to_dict(),
                        'capabilities': b.capabilities.to_dict(),
                        'request_transform': b.request_transform.to_dict(),
                        'response_transform': b.response_transform.to_dict(),
                        'gvl_vendor_id': b.gvl_vendor_id,
                        'priority': b.priority,
                    }
                    for b in bidders
                ],
                'count': len(bidders)
            })

        except Exception as e:
            return jsonify({
                'bidders': [],
                'error': 'Failed to list active bidders'
            }), 500

    return app


def run_admin(host: str = '0.0.0.0', port: int = 5050, debug: bool = False):
    """
    Run the admin dashboard.

    Args:
        host: Host to bind to (default: 0.0.0.0)
        port: Port to listen on (default: 5050)
        debug: Enable debug mode (default: False for security)

    Environment variables for authentication:
        ADMIN_USERS: Comma-separated user:pass pairs (e.g., "admin:pass123,ops:secret456")
        ADMIN_USER_1: First admin user (format: "username:password")
        ADMIN_USER_2: Second admin user (format: "username:password")
        ADMIN_USER_3: Third admin user (format: "username:password")
        SECRET_KEY: Flask secret key for sessions (auto-generated if not set)
    """
    app = create_app()
    print(f"\n{'='*60}")
    print("  IDR Admin Dashboard")
    print(f"{'='*60}")
    print(f"  URL: http://localhost:{port}")
    print(f"  Config: {DEFAULT_CONFIG_PATH}")
    print(f"  Debug: {debug}")
    print(f"{'='*60}\n")
    app.run(host=host, port=port, debug=debug)


if __name__ == '__main__':
    run_admin()
