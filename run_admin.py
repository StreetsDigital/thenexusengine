#!/usr/bin/env python3
"""
Run the IDR Admin Dashboard.

Usage:
    python run_admin.py
    python run_admin.py --port 8080
    python run_admin.py --no-debug
"""

import argparse
import sys


def main():
    parser = argparse.ArgumentParser(description="Run IDR Admin Dashboard")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to")
    parser.add_argument("--port", type=int, default=5050, help="Port to run on")
    parser.add_argument("--no-debug", action="store_true", help="Disable debug mode")
    args = parser.parse_args()

    try:
        from src.idr.admin import run_admin

        run_admin(host=args.host, port=args.port, debug=not args.no_debug)
    except ImportError as e:
        print(f"Error: {e}")
        print("\nMake sure Flask is installed:")
        print("  pip install flask pyyaml")
        sys.exit(1)


if __name__ == "__main__":
    main()
