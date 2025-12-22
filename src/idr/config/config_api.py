"""
Configuration API

REST API endpoints for managing hierarchical configurations.
Provides CRUD operations for publisher, site, and ad unit configurations.
"""

from datetime import datetime
from typing import Any

from flask import Blueprint, jsonify, request

from .config_resolver import (
    AdUnitConfig,
    PublisherConfigV2,
    SiteConfig,
    get_config_resolver,
)
from .feature_config import (
    ConfigLevel,
    FeatureConfig,
    IDRSettings,
)
from ..bidders.storage import get_bidder_storage
from ..bidders.models import (
    BidderConfig,
    BidderEndpoint,
    BidderStatus,
)


def create_config_api_blueprint() -> Blueprint:
    """Create Flask blueprint for configuration API."""
    bp = Blueprint("config_api", __name__, url_prefix="/api/v2/config")
    resolver = get_config_resolver()
    bidder_storage = get_bidder_storage()

    def _safe_error_response(error: Exception, message: str, status_code: int = 500):
        """Return a safe error response."""
        import logging

        logging.error(f"{message}: {error}", exc_info=True)
        return jsonify({"status": "error", "message": message}), status_code

    # =========================================
    # Global Configuration
    # =========================================

    @bp.route("/global", methods=["GET"])
    def get_global_config():
        """Get the global default configuration."""
        config = resolver.get_global_config()
        return jsonify(
            {
                "status": "success",
                "config": config.to_dict(),
            }
        )

    @bp.route("/global", methods=["PUT"])
    def update_global_config():
        """Update the global default configuration."""
        try:
            data = request.json
            config = FeatureConfig.from_dict(data)
            config.config_id = "global"
            config.config_level = ConfigLevel.GLOBAL
            config.updated_at = datetime.now()

            resolver.set_global_config(config)

            return jsonify(
                {
                    "status": "success",
                    "message": "Global configuration updated",
                    "config": config.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to update global configuration", 400)

    @bp.route("/global/idr", methods=["PATCH"])
    def update_global_idr():
        """Update only the IDR settings in global config."""
        try:
            data = request.json
            config = resolver.get_global_config()
            config.idr = IDRSettings.from_dict(data)
            config.updated_at = datetime.now()
            resolver.set_global_config(config)

            return jsonify(
                {
                    "status": "success",
                    "message": "Global IDR settings updated",
                    "idr": config.idr.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to update global IDR settings", 400)

    # =========================================
    # Publisher Configuration
    # =========================================

    @bp.route("/publishers", methods=["GET"])
    def list_publishers():
        """List all registered publishers."""
        publishers = []
        for _pub_id, config in resolver._publisher_configs.items():
            publishers.append(
                {
                    "publisher_id": config.publisher_id,
                    "name": config.name,
                    "enabled": config.enabled,
                    "sites_count": len(config.sites),
                    "has_custom_config": bool(config.features.config_id),
                }
            )

        return jsonify(
            {
                "status": "success",
                "publishers": publishers,
                "total": len(publishers),
            }
        )

    @bp.route("/publishers/<publisher_id>", methods=["GET"])
    def get_publisher_config(publisher_id: str):
        """Get configuration for a specific publisher."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        return jsonify(
            {
                "status": "success",
                "config": config.to_dict(),
            }
        )

    @bp.route("/publishers/<publisher_id>", methods=["PUT"])
    def save_publisher_config(publisher_id: str):
        """Save/update a publisher configuration."""
        try:
            data = request.json

            # Parse features if provided
            features = FeatureConfig()
            if "features" in data:
                features = FeatureConfig.from_dict(data["features"])
            features.config_id = f"publisher:{publisher_id}"
            features.config_level = ConfigLevel.PUBLISHER
            features.updated_at = datetime.now()

            # Parse sites
            sites = []
            for site_data in data.get("sites", []):
                site = _parse_site_from_dict(site_data)
                sites.append(site)

            config = PublisherConfigV2(
                publisher_id=publisher_id,
                name=data.get("name", publisher_id),
                enabled=data.get("enabled", True),
                contact_email=data.get("contact_email", ""),
                contact_name=data.get("contact_name", ""),
                sites=sites,
                bidders=data.get("bidders", {}),
                api_key=data.get("api_key", ""),
                features=features,
            )

            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Publisher {publisher_id} saved",
                    "config": config.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(
                e, "Failed to save publisher configuration", 400
            )

    @bp.route("/publishers/<publisher_id>", methods=["DELETE"])
    def delete_publisher_config(publisher_id: str):
        """Delete a publisher configuration."""
        if publisher_id not in resolver._publisher_configs:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        del resolver._publisher_configs[publisher_id]
        resolver._clear_publisher_cache(publisher_id)

        return jsonify(
            {
                "status": "success",
                "message": f"Publisher {publisher_id} deleted",
            }
        )

    @bp.route("/publishers/<publisher_id>/features", methods=["GET"])
    def get_publisher_features(publisher_id: str):
        """Get the feature configuration for a publisher."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        return jsonify(
            {
                "status": "success",
                "features": config.features.to_dict(),
            }
        )

    @bp.route("/publishers/<publisher_id>/features", methods=["PUT"])
    def update_publisher_features(publisher_id: str):
        """Update the feature configuration for a publisher."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            data = request.json
            features = FeatureConfig.from_dict(data)
            features.config_id = f"publisher:{publisher_id}"
            features.config_level = ConfigLevel.PUBLISHER
            features.updated_at = datetime.now()

            config.features = features
            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Publisher {publisher_id} features updated",
                    "features": features.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to update publisher features", 400)

    @bp.route("/publishers/<publisher_id>/idr", methods=["PATCH"])
    def update_publisher_idr(publisher_id: str):
        """Update only the IDR settings for a publisher."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            data = request.json
            config.features.idr = IDRSettings.from_dict(data)
            config.features.config_id = f"publisher:{publisher_id}"
            config.features.updated_at = datetime.now()
            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Publisher {publisher_id} IDR settings updated",
                    "idr": config.features.idr.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(
                e, "Failed to update publisher IDR settings", 400
            )

    # =========================================
    # Publisher Bidder Configuration
    # =========================================

    @bp.route("/publishers/<publisher_id>/bidders", methods=["GET"])
    def list_publisher_bidders(publisher_id: str):
        """
        List all bidders available for a publisher.

        Returns global bidders plus publisher-specific custom instances,
        grouped by bidder family.
        """
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        # Get bidders grouped by family
        families = bidder_storage.get_bidder_families(publisher_id)
        enabled_codes = bidder_storage.get_enabled_bidders_for_publisher(publisher_id)

        # Format response
        bidders_by_family = {}
        for family, instances in families.items():
            bidders_by_family[family] = {
                "family": family,
                "instance_count": len(instances),
                "instances": [
                    {
                        **inst.to_dict(),
                        "is_global": inst.publisher_id is None,
                        "is_enabled_for_publisher": inst.bidder_code in enabled_codes,
                    }
                    for inst in instances
                ],
            }

        return jsonify(
            {
                "status": "success",
                "publisher_id": publisher_id,
                "bidders_by_family": bidders_by_family,
                "total_families": len(bidders_by_family),
                "total_bidders": sum(len(f["instances"]) for f in bidders_by_family.values()),
            }
        )

    @bp.route("/publishers/<publisher_id>/bidders", methods=["POST"])
    def create_publisher_bidder(publisher_id: str):
        """
        Create a new publisher-specific bidder instance.

        Request body should contain the full bidder configuration.
        The bidder_code will be auto-generated if not provided.
        """
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            data = request.json

            # If bidder_family provided but no bidder_code, auto-generate
            bidder_family = data.get("bidder_family", data.get("bidder_code", ""))
            if not data.get("bidder_code"):
                data["bidder_code"] = bidder_storage.generate_instance_code(
                    publisher_id, bidder_family
                )

            # Parse endpoint if provided
            endpoint_data = data.get("endpoint", {"url": ""})
            endpoint = BidderEndpoint.from_dict(endpoint_data)

            bidder_config = BidderConfig(
                bidder_code=data["bidder_code"],
                name=data.get("name", data["bidder_code"]),
                endpoint=endpoint,
                bidder_family=bidder_family,
                publisher_id=publisher_id,
            )

            # Apply additional fields from request
            if "description" in data:
                bidder_config.description = data["description"]
            if "capabilities" in data:
                from ..bidders.models import BidderCapabilities
                bidder_config.capabilities = BidderCapabilities.from_dict(data["capabilities"])
            if "rate_limits" in data:
                from ..bidders.models import BidderRateLimits
                bidder_config.rate_limits = BidderRateLimits.from_dict(data["rate_limits"])
            if "request_transform" in data:
                from ..bidders.models import RequestTransform
                bidder_config.request_transform = RequestTransform.from_dict(data["request_transform"])
            if "response_transform" in data:
                from ..bidders.models import ResponseTransform
                bidder_config.response_transform = ResponseTransform.from_dict(data["response_transform"])
            if "status" in data:
                bidder_config.status = BidderStatus(data["status"])
            if "gvl_vendor_id" in data:
                bidder_config.gvl_vendor_id = data["gvl_vendor_id"]
            if "priority" in data:
                bidder_config.priority = data["priority"]
            if "allowed_countries" in data:
                bidder_config.allowed_countries = data["allowed_countries"]
            if "blocked_countries" in data:
                bidder_config.blocked_countries = data["blocked_countries"]

            if bidder_storage.save_publisher_bidder(publisher_id, bidder_config):
                return jsonify(
                    {
                        "status": "success",
                        "message": f"Bidder {bidder_config.bidder_code} created",
                        "bidder": bidder_config.to_dict(),
                    }
                )
            else:
                return jsonify(
                    {
                        "status": "error",
                        "message": "Failed to save bidder configuration",
                    }
                ), 500

        except Exception as e:
            return _safe_error_response(e, "Failed to create publisher bidder", 400)

    @bp.route("/publishers/<publisher_id>/bidders/<bidder_code>", methods=["GET"])
    def get_publisher_bidder(publisher_id: str, bidder_code: str):
        """Get a specific bidder configuration for a publisher."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        # Try publisher-specific first, then global
        bidder = bidder_storage.get_publisher_bidder(publisher_id, bidder_code)
        is_global = False
        if not bidder:
            bidder = bidder_storage.get(bidder_code)
            is_global = True

        if not bidder:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Bidder {bidder_code} not found",
                }
            ), 404

        enabled_codes = bidder_storage.get_enabled_bidders_for_publisher(publisher_id)

        return jsonify(
            {
                "status": "success",
                "bidder": {
                    **bidder.to_dict(),
                    "is_global": is_global,
                    "is_enabled_for_publisher": bidder_code in enabled_codes,
                },
            }
        )

    @bp.route("/publishers/<publisher_id>/bidders/<bidder_code>", methods=["PUT"])
    def update_publisher_bidder(publisher_id: str, bidder_code: str):
        """Update a publisher-specific bidder configuration."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            # Get existing bidder (publisher-specific or global)
            existing = bidder_storage.get_publisher_bidder(publisher_id, bidder_code)
            is_global = False
            if not existing:
                existing = bidder_storage.get(bidder_code)
                is_global = True

            if not existing:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Bidder {bidder_code} not found",
                    }
                ), 404

            # If global bidder, create a publisher-specific copy
            if is_global:
                import copy
                existing = copy.deepcopy(existing)
                existing.publisher_id = publisher_id

            data = request.json

            # Update fields from request
            if "name" in data:
                existing.name = data["name"]
            if "description" in data:
                existing.description = data["description"]
            if "endpoint" in data:
                existing.endpoint = BidderEndpoint.from_dict(data["endpoint"])
            if "capabilities" in data:
                from ..bidders.models import BidderCapabilities
                existing.capabilities = BidderCapabilities.from_dict(data["capabilities"])
            if "rate_limits" in data:
                from ..bidders.models import BidderRateLimits
                existing.rate_limits = BidderRateLimits.from_dict(data["rate_limits"])
            if "request_transform" in data:
                from ..bidders.models import RequestTransform
                existing.request_transform = RequestTransform.from_dict(data["request_transform"])
            if "response_transform" in data:
                from ..bidders.models import ResponseTransform
                existing.response_transform = ResponseTransform.from_dict(data["response_transform"])
            if "status" in data:
                existing.status = BidderStatus(data["status"])
            if "gvl_vendor_id" in data:
                existing.gvl_vendor_id = data["gvl_vendor_id"]
            if "priority" in data:
                existing.priority = data["priority"]
            if "allowed_countries" in data:
                existing.allowed_countries = data["allowed_countries"]
            if "blocked_countries" in data:
                existing.blocked_countries = data["blocked_countries"]

            if bidder_storage.save_publisher_bidder(publisher_id, existing):
                return jsonify(
                    {
                        "status": "success",
                        "message": f"Bidder {bidder_code} updated",
                        "bidder": existing.to_dict(),
                    }
                )
            else:
                return jsonify(
                    {
                        "status": "error",
                        "message": "Failed to save bidder configuration",
                    }
                ), 500

        except Exception as e:
            return _safe_error_response(e, "Failed to update publisher bidder", 400)

    @bp.route("/publishers/<publisher_id>/bidders/<bidder_code>", methods=["DELETE"])
    def delete_publisher_bidder(publisher_id: str, bidder_code: str):
        """Delete a publisher-specific bidder configuration."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        # Check if this is a publisher-specific bidder
        bidder = bidder_storage.get_publisher_bidder(publisher_id, bidder_code)
        if not bidder:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher-specific bidder {bidder_code} not found. "
                    "Cannot delete global bidders from publisher context.",
                }
            ), 404

        if bidder_storage.delete_publisher_bidder(publisher_id, bidder_code):
            return jsonify(
                {
                    "status": "success",
                    "message": f"Bidder {bidder_code} deleted",
                }
            )
        else:
            return jsonify(
                {
                    "status": "error",
                    "message": "Failed to delete bidder configuration",
                }
            ), 500

    @bp.route("/publishers/<publisher_id>/bidders/<bidder_code>/enable", methods=["POST"])
    def enable_publisher_bidder(publisher_id: str, bidder_code: str):
        """Enable a bidder for a specific publisher."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        if bidder_storage.set_bidder_enabled_for_publisher(publisher_id, bidder_code, True):
            return jsonify(
                {
                    "status": "success",
                    "message": f"Bidder {bidder_code} enabled for publisher {publisher_id}",
                }
            )
        else:
            return jsonify(
                {
                    "status": "error",
                    "message": "Failed to enable bidder",
                }
            ), 500

    @bp.route("/publishers/<publisher_id>/bidders/<bidder_code>/disable", methods=["POST"])
    def disable_publisher_bidder(publisher_id: str, bidder_code: str):
        """Disable a bidder for a specific publisher."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        if bidder_storage.set_bidder_enabled_for_publisher(publisher_id, bidder_code, False):
            return jsonify(
                {
                    "status": "success",
                    "message": f"Bidder {bidder_code} disabled for publisher {publisher_id}",
                }
            )
        else:
            return jsonify(
                {
                    "status": "error",
                    "message": "Failed to disable bidder",
                }
            ), 500

    @bp.route("/publishers/<publisher_id>/bidders/duplicate", methods=["POST"])
    def duplicate_publisher_bidder(publisher_id: str):
        """
        Duplicate a bidder with auto-generated instance code.

        Request body:
        {
            "source_bidder_code": "appnexus",
            "name": "Optional new name"
        }
        """
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            data = request.json
            source_code = data.get("source_bidder_code")
            new_name = data.get("name")

            if not source_code:
                return jsonify(
                    {
                        "status": "error",
                        "message": "source_bidder_code is required",
                    }
                ), 400

            new_bidder = bidder_storage.duplicate_bidder(publisher_id, source_code, new_name)

            if new_bidder:
                return jsonify(
                    {
                        "status": "success",
                        "message": f"Bidder duplicated as {new_bidder.bidder_code}",
                        "bidder": new_bidder.to_dict(),
                    }
                )
            else:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Source bidder {source_code} not found",
                    }
                ), 404

        except Exception as e:
            return _safe_error_response(e, "Failed to duplicate bidder", 400)

    @bp.route("/publishers/<publisher_id>/bidders/families", methods=["GET"])
    def get_publisher_bidder_families(publisher_id: str):
        """
        Get bidders grouped by family for UI display.
        Returns bidders organized by their base family (e.g., appnexus, rubicon).
        """
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            families = bidder_storage.get_bidder_families(publisher_id)
            enabled = bidder_storage.get_enabled_bidders_for_publisher(publisher_id)

            return jsonify(
                {
                    "status": "success",
                    "families": families,
                    "enabled": list(enabled),  # Convert set to list for JSON serialization
                }
            )

        except Exception as e:
            return _safe_error_response(e, "Failed to get bidder families", 500)

    # =========================================
    # Site Configuration
    # =========================================

    @bp.route("/publishers/<publisher_id>/sites", methods=["GET"])
    def list_sites(publisher_id: str):
        """List all sites for a publisher."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        sites = []
        for site in config.sites:
            sites.append(
                {
                    "site_id": site.site_id,
                    "domain": site.domain,
                    "name": site.name,
                    "enabled": site.enabled,
                    "ad_units_count": len(site.ad_units),
                    "has_custom_config": bool(site.features.config_id),
                }
            )

        return jsonify(
            {
                "status": "success",
                "sites": sites,
                "total": len(sites),
            }
        )

    @bp.route("/publishers/<publisher_id>/sites/<site_id>", methods=["GET"])
    def get_site_config(publisher_id: str, site_id: str):
        """Get configuration for a specific site."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        site = config.get_site(site_id)
        if not site:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Site {site_id} not found",
                }
            ), 404

        return jsonify(
            {
                "status": "success",
                "config": site.to_dict(),
            }
        )

    @bp.route("/publishers/<publisher_id>/sites/<site_id>", methods=["PUT"])
    def save_site_config(publisher_id: str, site_id: str):
        """Save/update a site configuration."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            data = request.json
            site = _parse_site_from_dict(data)
            site.site_id = site_id

            # Update or add site
            site_found = False
            for i, s in enumerate(config.sites):
                if s.site_id == site_id:
                    config.sites[i] = site
                    site_found = True
                    break

            if not site_found:
                config.sites.append(site)

            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Site {site_id} saved",
                    "config": site.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to save site configuration", 400)

    @bp.route("/publishers/<publisher_id>/sites/<site_id>", methods=["DELETE"])
    def delete_site_config(publisher_id: str, site_id: str):
        """Delete a site configuration."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        config.sites = [s for s in config.sites if s.site_id != site_id]
        resolver.register_publisher(config)

        return jsonify(
            {
                "status": "success",
                "message": f"Site {site_id} deleted",
            }
        )

    @bp.route("/publishers/<publisher_id>/sites/<site_id>/features", methods=["GET"])
    def get_site_features(publisher_id: str, site_id: str):
        """Get the feature configuration for a site."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        site = config.get_site(site_id)
        if not site:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Site {site_id} not found",
                }
            ), 404

        return jsonify(
            {
                "status": "success",
                "features": site.features.to_dict(),
            }
        )

    @bp.route("/publishers/<publisher_id>/sites/<site_id>/features", methods=["PUT"])
    def update_site_features(publisher_id: str, site_id: str):
        """Update the feature configuration for a site."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            site = config.get_site(site_id)
            if not site:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Site {site_id} not found",
                    }
                ), 404

            data = request.json
            features = FeatureConfig.from_dict(data)
            features.config_id = f"site:{site_id}"
            features.config_level = ConfigLevel.SITE
            features.updated_at = datetime.now()

            site.features = features
            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Site {site_id} features updated",
                    "features": features.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to update site features", 400)

    @bp.route("/publishers/<publisher_id>/sites/<site_id>/idr", methods=["PATCH"])
    def update_site_idr(publisher_id: str, site_id: str):
        """Update only the IDR settings for a site."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            site = config.get_site(site_id)
            if not site:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Site {site_id} not found",
                    }
                ), 404

            data = request.json
            site.features.idr = IDRSettings.from_dict(data)
            site.features.config_id = f"site:{site_id}"
            site.features.updated_at = datetime.now()
            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Site {site_id} IDR settings updated",
                    "idr": site.features.idr.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to update site IDR settings", 400)

    # =========================================
    # Ad Unit Configuration
    # =========================================

    @bp.route("/publishers/<publisher_id>/sites/<site_id>/ad-units", methods=["GET"])
    def list_ad_units(publisher_id: str, site_id: str):
        """List all ad units for a site."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        site = config.get_site(site_id)
        if not site:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Site {site_id} not found",
                }
            ), 404

        ad_units = []
        for unit in site.ad_units:
            ad_units.append(
                {
                    "unit_id": unit.unit_id,
                    "name": unit.name,
                    "enabled": unit.enabled,
                    "media_type": unit.media_type,
                    "sizes": unit.sizes,
                    "position": unit.position,
                    "has_custom_config": bool(unit.features.config_id),
                }
            )

        return jsonify(
            {
                "status": "success",
                "ad_units": ad_units,
                "total": len(ad_units),
            }
        )

    @bp.route(
        "/publishers/<publisher_id>/sites/<site_id>/ad-units/<unit_id>", methods=["GET"]
    )
    def get_ad_unit_config(publisher_id: str, site_id: str, unit_id: str):
        """Get configuration for a specific ad unit."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        ad_unit = config.get_ad_unit(site_id, unit_id)
        if not ad_unit:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Ad unit {unit_id} not found",
                }
            ), 404

        return jsonify(
            {
                "status": "success",
                "config": ad_unit.to_dict(),
            }
        )

    @bp.route(
        "/publishers/<publisher_id>/sites/<site_id>/ad-units/<unit_id>", methods=["PUT"]
    )
    def save_ad_unit_config(publisher_id: str, site_id: str, unit_id: str):
        """Save/update an ad unit configuration."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            site = config.get_site(site_id)
            if not site:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Site {site_id} not found",
                    }
                ), 404

            data = request.json
            ad_unit = _parse_ad_unit_from_dict(data)
            ad_unit.unit_id = unit_id

            # Update or add ad unit
            unit_found = False
            for i, u in enumerate(site.ad_units):
                if u.unit_id == unit_id:
                    site.ad_units[i] = ad_unit
                    unit_found = True
                    break

            if not unit_found:
                site.ad_units.append(ad_unit)

            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Ad unit {unit_id} saved",
                    "config": ad_unit.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to save ad unit configuration", 400)

    @bp.route(
        "/publishers/<publisher_id>/sites/<site_id>/ad-units/<unit_id>",
        methods=["DELETE"],
    )
    def delete_ad_unit_config(publisher_id: str, site_id: str, unit_id: str):
        """Delete an ad unit configuration."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        site = config.get_site(site_id)
        if not site:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Site {site_id} not found",
                }
            ), 404

        site.ad_units = [u for u in site.ad_units if u.unit_id != unit_id]
        resolver.register_publisher(config)

        return jsonify(
            {
                "status": "success",
                "message": f"Ad unit {unit_id} deleted",
            }
        )

    @bp.route(
        "/publishers/<publisher_id>/sites/<site_id>/ad-units/<unit_id>/features",
        methods=["GET"],
    )
    def get_ad_unit_features(publisher_id: str, site_id: str, unit_id: str):
        """Get the feature configuration for an ad unit."""
        config = resolver.get_publisher_config(publisher_id)
        if not config:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Publisher {publisher_id} not found",
                }
            ), 404

        ad_unit = config.get_ad_unit(site_id, unit_id)
        if not ad_unit:
            return jsonify(
                {
                    "status": "error",
                    "message": f"Ad unit {unit_id} not found",
                }
            ), 404

        return jsonify(
            {
                "status": "success",
                "features": ad_unit.features.to_dict(),
            }
        )

    @bp.route(
        "/publishers/<publisher_id>/sites/<site_id>/ad-units/<unit_id>/features",
        methods=["PUT"],
    )
    def update_ad_unit_features(publisher_id: str, site_id: str, unit_id: str):
        """Update the feature configuration for an ad unit."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            site = config.get_site(site_id)
            if not site:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Site {site_id} not found",
                    }
                ), 404

            ad_unit = None
            for u in site.ad_units:
                if u.unit_id == unit_id:
                    ad_unit = u
                    break

            if not ad_unit:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Ad unit {unit_id} not found",
                    }
                ), 404

            data = request.json
            features = FeatureConfig.from_dict(data)
            features.config_id = f"ad_unit:{unit_id}"
            features.config_level = ConfigLevel.AD_UNIT
            features.updated_at = datetime.now()

            ad_unit.features = features
            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Ad unit {unit_id} features updated",
                    "features": features.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to update ad unit features", 400)

    @bp.route(
        "/publishers/<publisher_id>/sites/<site_id>/ad-units/<unit_id>/idr",
        methods=["PATCH"],
    )
    def update_ad_unit_idr(publisher_id: str, site_id: str, unit_id: str):
        """Update only the IDR settings for an ad unit."""
        try:
            config = resolver.get_publisher_config(publisher_id)
            if not config:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Publisher {publisher_id} not found",
                    }
                ), 404

            site = config.get_site(site_id)
            if not site:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Site {site_id} not found",
                    }
                ), 404

            ad_unit = None
            for u in site.ad_units:
                if u.unit_id == unit_id:
                    ad_unit = u
                    break

            if not ad_unit:
                return jsonify(
                    {
                        "status": "error",
                        "message": f"Ad unit {unit_id} not found",
                    }
                ), 404

            data = request.json
            ad_unit.features.idr = IDRSettings.from_dict(data)
            ad_unit.features.config_id = f"ad_unit:{unit_id}"
            ad_unit.features.updated_at = datetime.now()
            resolver.register_publisher(config)

            return jsonify(
                {
                    "status": "success",
                    "message": f"Ad unit {unit_id} IDR settings updated",
                    "idr": ad_unit.features.idr.to_dict(),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to update ad unit IDR settings", 400)

    # =========================================
    # Configuration Resolution
    # =========================================

    @bp.route("/resolve", methods=["GET"])
    def resolve_configuration():
        """
        Resolve the effective configuration for a given context.

        Query params:
            publisher_id: (optional) Publisher ID
            site_id: (optional) Site ID
            unit_id: (optional) Ad unit ID

        Returns the fully resolved configuration with inheritance applied.
        """
        publisher_id = request.args.get("publisher_id")
        site_id = request.args.get("site_id")
        unit_id = request.args.get("unit_id")

        resolved = resolver.resolve(publisher_id, site_id, unit_id)

        return jsonify(
            {
                "status": "success",
                "resolved": resolved.to_dict(),
                "resolution_chain": resolved.resolution_chain,
            }
        )

    @bp.route("/resolve/idr", methods=["GET"])
    def resolve_idr_settings():
        """
        Resolve just the IDR settings for a given context.

        Useful for quick lookups of effective IDR configuration.
        """
        publisher_id = request.args.get("publisher_id")
        site_id = request.args.get("site_id")
        unit_id = request.args.get("unit_id")

        resolved = resolver.resolve(publisher_id, site_id, unit_id)

        return jsonify(
            {
                "status": "success",
                "idr": {
                    "enabled": resolved.idr_enabled,
                    "bypass_enabled": resolved.bypass_enabled,
                    "shadow_mode": resolved.shadow_mode,
                    "max_bidders": resolved.max_bidders,
                    "min_score_threshold": resolved.min_score_threshold,
                    "exploration_enabled": resolved.exploration_enabled,
                    "exploration_rate": resolved.exploration_rate,
                    "exploration_slots": resolved.exploration_slots,
                    "anchor_bidders_enabled": resolved.anchor_bidders_enabled,
                    "anchor_bidder_count": resolved.anchor_bidder_count,
                    "custom_anchor_bidders": resolved.custom_anchor_bidders,
                    "diversity_enabled": resolved.diversity_enabled,
                    "diversity_categories": resolved.diversity_categories,
                    "scoring_weights": resolved.scoring_weights,
                    "latency_excellent_ms": resolved.latency_excellent_ms,
                    "latency_poor_ms": resolved.latency_poor_ms,
                    "selection_timeout_ms": resolved.selection_timeout_ms,
                },
                "resolution_chain": resolved.resolution_chain,
            }
        )

    # =========================================
    # Bulk Operations
    # =========================================

    @bp.route("/bulk/idr", methods=["PATCH"])
    def bulk_update_idr():
        """
        Bulk update IDR settings across multiple entities.

        Request body:
        {
            "updates": [
                {
                    "publisher_id": "pub1",
                    "site_id": "site1",  // optional
                    "unit_id": "unit1",  // optional
                    "idr": { ... idr settings ... }
                },
                ...
            ]
        }
        """
        try:
            data = request.json
            updates = data.get("updates", [])
            results = []

            for update in updates:
                publisher_id = update.get("publisher_id")
                site_id = update.get("site_id")
                unit_id = update.get("unit_id")
                idr_data = update.get("idr", {})

                try:
                    config = resolver.get_publisher_config(publisher_id)
                    if not config:
                        results.append(
                            {
                                "entity": _make_entity_key(
                                    publisher_id, site_id, unit_id
                                ),
                                "status": "error",
                                "message": f"Publisher {publisher_id} not found",
                            }
                        )
                        continue

                    if unit_id and site_id:
                        # Update ad unit
                        site = config.get_site(site_id)
                        if not site:
                            results.append(
                                {
                                    "entity": _make_entity_key(
                                        publisher_id, site_id, unit_id
                                    ),
                                    "status": "error",
                                    "message": f"Site {site_id} not found",
                                }
                            )
                            continue

                        ad_unit = None
                        for u in site.ad_units:
                            if u.unit_id == unit_id:
                                ad_unit = u
                                break

                        if not ad_unit:
                            results.append(
                                {
                                    "entity": _make_entity_key(
                                        publisher_id, site_id, unit_id
                                    ),
                                    "status": "error",
                                    "message": f"Ad unit {unit_id} not found",
                                }
                            )
                            continue

                        ad_unit.features.idr = IDRSettings.from_dict(idr_data)
                        ad_unit.features.config_id = f"ad_unit:{unit_id}"

                    elif site_id:
                        # Update site
                        site = config.get_site(site_id)
                        if not site:
                            results.append(
                                {
                                    "entity": _make_entity_key(
                                        publisher_id, site_id, unit_id
                                    ),
                                    "status": "error",
                                    "message": f"Site {site_id} not found",
                                }
                            )
                            continue

                        site.features.idr = IDRSettings.from_dict(idr_data)
                        site.features.config_id = f"site:{site_id}"

                    else:
                        # Update publisher
                        config.features.idr = IDRSettings.from_dict(idr_data)
                        config.features.config_id = f"publisher:{publisher_id}"

                    resolver.register_publisher(config)
                    results.append(
                        {
                            "entity": _make_entity_key(publisher_id, site_id, unit_id),
                            "status": "success",
                        }
                    )

                except Exception as e:
                    results.append(
                        {
                            "entity": _make_entity_key(publisher_id, site_id, unit_id),
                            "status": "error",
                            "message": str(e),
                        }
                    )

            return jsonify(
                {
                    "status": "success",
                    "results": results,
                    "total": len(results),
                    "successful": sum(1 for r in results if r["status"] == "success"),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to perform bulk update", 400)

    @bp.route("/cache/clear", methods=["POST"])
    def clear_cache():
        """Clear all cached configuration resolutions."""
        resolver.clear_cache()
        return jsonify(
            {
                "status": "success",
                "message": "Configuration cache cleared",
            }
        )

    # =========================================
    # Persistence Operations
    # =========================================

    @bp.route("/sync/to-db", methods=["POST"])
    def sync_to_db():
        """
        Sync all in-memory configurations to PostgreSQL.

        Use this to ensure all configs are persisted.
        """
        try:
            success = resolver.sync_to_store()
            if success:
                return jsonify(
                    {
                        "status": "success",
                        "message": "All configurations synced to PostgreSQL",
                    }
                )
            else:
                return jsonify(
                    {
                        "status": "error",
                        "message": "Sync failed - persistence may be disabled",
                    }
                ), 500
        except Exception as e:
            return _safe_error_response(e, "Failed to sync to PostgreSQL", 500)

    @bp.route("/sync/from-db", methods=["POST"])
    def sync_from_db():
        """
        Reload all configurations from PostgreSQL.

        Use this to refresh in-memory configs from database.
        """
        try:
            success = resolver.sync_from_store()
            if success:
                return jsonify(
                    {
                        "status": "success",
                        "message": "Configurations reloaded from PostgreSQL",
                    }
                )
            else:
                return jsonify(
                    {
                        "status": "error",
                        "message": "Reload failed",
                    }
                ), 500
        except Exception as e:
            return _safe_error_response(e, "Failed to reload from PostgreSQL", 500)

    @bp.route("/export", methods=["GET"])
    def export_configs():
        """
        Export all configurations as JSON.

        Returns all global, publisher, site, and ad unit configurations.
        """
        try:
            data = resolver.export_configs()
            return jsonify(
                {
                    "status": "success",
                    "data": data,
                    "publishers_count": len(data.get("publishers", {})),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to export configurations", 500)

    @bp.route("/import", methods=["POST"])
    def import_configs():
        """
        Import configurations from JSON.

        Request body should contain 'global' and 'publishers' keys.
        This will overwrite existing configurations.
        """
        try:
            data = request.json
            if not data:
                return jsonify(
                    {
                        "status": "error",
                        "message": "No data provided",
                    }
                ), 400

            success = resolver.import_configs(data)
            if success:
                return jsonify(
                    {
                        "status": "success",
                        "message": "Configurations imported successfully",
                        "publishers_imported": len(data.get("publishers", {})),
                    }
                )
            else:
                return jsonify(
                    {
                        "status": "error",
                        "message": "Import failed",
                    }
                ), 500
        except Exception as e:
            return _safe_error_response(e, "Failed to import configurations", 400)

    @bp.route("/storage/status", methods=["GET"])
    def storage_status():
        """
        Get the status of the configuration storage (PostgreSQL).
        """
        try:
            from .config_store import get_config_store

            store = get_config_store()

            return jsonify(
                {
                    "status": "success",
                    "postgres_connected": store.is_connected,
                    "using_memory_fallback": store._use_memory,
                    "publishers_stored": len(store.list_publishers()),
                }
            )
        except Exception as e:
            return _safe_error_response(e, "Failed to get storage status", 500)

    return bp


def _parse_site_from_dict(data: dict[str, Any]) -> SiteConfig:
    """Parse a site configuration from dictionary."""
    features = FeatureConfig()
    if "features" in data:
        features = FeatureConfig.from_dict(data["features"])
        features.config_level = ConfigLevel.SITE

    ad_units = []
    for unit_data in data.get("ad_units", []):
        ad_units.append(_parse_ad_unit_from_dict(unit_data))

    return SiteConfig(
        site_id=data.get("site_id", ""),
        domain=data.get("domain", ""),
        name=data.get("name", ""),
        enabled=data.get("enabled", True),
        ad_units=ad_units,
        features=features,
    )


def _parse_ad_unit_from_dict(data: dict[str, Any]) -> AdUnitConfig:
    """Parse an ad unit configuration from dictionary."""
    features = FeatureConfig()
    if "features" in data:
        features = FeatureConfig.from_dict(data["features"])
        features.config_level = ConfigLevel.AD_UNIT

    return AdUnitConfig(
        unit_id=data.get("unit_id", ""),
        name=data.get("name", ""),
        enabled=data.get("enabled", True),
        sizes=data.get("sizes", []),
        media_type=data.get("media_type", "banner"),
        position=data.get("position", "unknown"),
        floor_price=data.get("floor_price"),
        floor_currency=data.get("floor_currency", "USD"),
        video=data.get("video"),
        features=features,
    )


def _make_entity_key(
    publisher_id: str, site_id: str | None, unit_id: str | None
) -> str:
    """Create a unique key for an entity."""
    if unit_id and site_id:
        return f"{publisher_id}/{site_id}/{unit_id}"
    elif site_id:
        return f"{publisher_id}/{site_id}"
    else:
        return publisher_id
