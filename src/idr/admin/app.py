"""
IDR Admin Dashboard - Simple web UI for managing IDR configuration.

Run with: python -m src.idr.admin.app
"""

import json
import os
from datetime import datetime
from pathlib import Path
from typing import Any, Optional

import yaml

try:
    from flask import Flask, jsonify, render_template, request
except ImportError:
    print("Flask not installed. Run: pip install flask")
    raise


# Default config path
DEFAULT_CONFIG_PATH = Path(__file__).parent.parent.parent.parent.parent / "config" / "idr_config.yaml"


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
            'bypass_enabled': False,
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
    }


def create_app(config_path: Optional[Path] = None) -> Flask:
    """Create and configure the Flask application."""
    app = Flask(
        __name__,
        template_folder=str(Path(__file__).parent / 'templates'),
        static_folder=str(Path(__file__).parent / 'static'),
    )

    app.config['CONFIG_PATH'] = config_path or DEFAULT_CONFIG_PATH

    @app.route('/')
    def index():
        """Main dashboard page."""
        config = load_config(app.config['CONFIG_PATH'])
        return render_template('index.html', config=config)

    @app.route('/api/config', methods=['GET'])
    def get_config():
        """Get current configuration."""
        config = load_config(app.config['CONFIG_PATH'])
        return jsonify(config)

    @app.route('/api/config', methods=['POST'])
    def update_config():
        """Update configuration."""
        try:
            new_config = request.json
            save_config(new_config, app.config['CONFIG_PATH'])
            return jsonify({'status': 'success', 'message': 'Configuration saved'})
        except Exception as e:
            return jsonify({'status': 'error', 'message': str(e)}), 400

    @app.route('/api/config/selector', methods=['PATCH'])
    def update_selector():
        """Update selector settings only."""
        try:
            config = load_config(app.config['CONFIG_PATH'])
            updates = request.json
            config['selector'].update(updates)
            save_config(config, app.config['CONFIG_PATH'])
            return jsonify({'status': 'success', 'config': config['selector']})
        except Exception as e:
            return jsonify({'status': 'error', 'message': str(e)}), 400

    @app.route('/api/config/scoring', methods=['PATCH'])
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
            return jsonify({'status': 'error', 'message': str(e)}), 400

    @app.route('/api/mode/bypass', methods=['POST'])
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
            return jsonify({'status': 'error', 'message': str(e)}), 400

    @app.route('/api/mode/shadow', methods=['POST'])
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
            return jsonify({'status': 'error', 'message': str(e)}), 400

    @app.route('/api/reset', methods=['POST'])
    def reset_config():
        """Reset to default configuration."""
        try:
            default = get_default_config()
            save_config(default, app.config['CONFIG_PATH'])
            return jsonify({'status': 'success', 'message': 'Configuration reset to defaults'})
        except Exception as e:
            return jsonify({'status': 'error', 'message': str(e)}), 400

    return app


def run_admin(host: str = '0.0.0.0', port: int = 5050, debug: bool = True):
    """Run the admin dashboard."""
    app = create_app()
    print(f"\n{'='*60}")
    print("  IDR Admin Dashboard")
    print(f"{'='*60}")
    print(f"  URL: http://localhost:{port}")
    print(f"  Config: {DEFAULT_CONFIG_PATH}")
    print(f"{'='*60}\n")
    app.run(host=host, port=port, debug=debug)


if __name__ == '__main__':
    run_admin()
