/**
 * API Client for IDR Admin
 * Provides a clean interface for all API operations
 *
 * Usage:
 *   import { API } from './api.js';
 *   const publishers = await API.publishers.list();
 */

const BASE_URL = '/api/v2/config';

/**
 * Helper function to handle API responses
 */
async function handleResponse(response) {
    if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        throw new Error(error.message || `API error: ${response.status}`);
    }
    return response.json();
}

/**
 * Helper function to make API requests
 */
async function request(url, options = {}) {
    const response = await fetch(url, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...options.headers,
        },
    });
    return handleResponse(response);
}

/**
 * Publisher API
 */
export const publishers = {
    /**
     * List all publishers
     * @returns {Promise<{publishers: Array}>}
     */
    async list() {
        return request(`${BASE_URL}/publishers`);
    },

    /**
     * Get a single publisher by ID
     * @param {string} id - Publisher ID
     * @returns {Promise<Object>}
     */
    async get(id) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(id)}`);
    },

    /**
     * Create or update a publisher
     * @param {string} id - Publisher ID
     * @param {Object} data - Publisher data
     * @returns {Promise<Object>}
     */
    async save(id, data) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(id)}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    },

    /**
     * Delete a publisher
     * @param {string} id - Publisher ID
     * @returns {Promise<Object>}
     */
    async delete(id) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(id)}`, {
            method: 'DELETE',
        });
    },

    /**
     * Get publisher features
     * @param {string} id - Publisher ID
     * @returns {Promise<Object>}
     */
    async getFeatures(id) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(id)}/features`);
    },

    /**
     * Save publisher features
     * @param {string} id - Publisher ID
     * @param {Object} data - Features data
     * @returns {Promise<Object>}
     */
    async saveFeatures(id, data) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(id)}/features`, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    },
};

/**
 * Site API
 */
export const sites = {
    /**
     * List sites for a publisher
     * @param {string} publisherId - Publisher ID
     * @returns {Promise<{sites: Array}>}
     */
    async list(publisherId) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites`);
    },

    /**
     * Get a single site
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @returns {Promise<Object>}
     */
    async get(publisherId, siteId) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}`
        );
    },

    /**
     * Create or update a site
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @param {Object} data - Site data
     * @returns {Promise<Object>}
     */
    async save(publisherId, siteId, data) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}`,
            {
                method: 'PUT',
                body: JSON.stringify(data),
            }
        );
    },

    /**
     * Delete a site
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @returns {Promise<Object>}
     */
    async delete(publisherId, siteId) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}`,
            {
                method: 'DELETE',
            }
        );
    },

    /**
     * Get site features
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @returns {Promise<Object>}
     */
    async getFeatures(publisherId, siteId) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/features`
        );
    },

    /**
     * Save site features
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @param {Object} data - Features data
     * @returns {Promise<Object>}
     */
    async saveFeatures(publisherId, siteId, data) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/features`,
            {
                method: 'PUT',
                body: JSON.stringify(data),
            }
        );
    },
};

/**
 * Ad Unit API
 */
export const adUnits = {
    /**
     * List ad units for a site
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @returns {Promise<{ad_units: Array}>}
     */
    async list(publisherId, siteId) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/ad-units`
        );
    },

    /**
     * Get a single ad unit
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @param {string} unitId - Ad unit ID
     * @returns {Promise<Object>}
     */
    async get(publisherId, siteId, unitId) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/ad-units/${encodeURIComponent(unitId)}`
        );
    },

    /**
     * Create or update an ad unit
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @param {string} unitId - Ad unit ID
     * @param {Object} data - Ad unit data
     * @returns {Promise<Object>}
     */
    async save(publisherId, siteId, unitId, data) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/ad-units/${encodeURIComponent(unitId)}`,
            {
                method: 'PUT',
                body: JSON.stringify(data),
            }
        );
    },

    /**
     * Delete an ad unit
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @param {string} unitId - Ad unit ID
     * @returns {Promise<Object>}
     */
    async delete(publisherId, siteId, unitId) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/ad-units/${encodeURIComponent(unitId)}`,
            {
                method: 'DELETE',
            }
        );
    },

    /**
     * Get ad unit features
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @param {string} unitId - Ad unit ID
     * @returns {Promise<Object>}
     */
    async getFeatures(publisherId, siteId, unitId) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/ad-units/${encodeURIComponent(unitId)}/features`
        );
    },

    /**
     * Save ad unit features
     * @param {string} publisherId - Publisher ID
     * @param {string} siteId - Site ID
     * @param {string} unitId - Ad unit ID
     * @param {Object} data - Features data
     * @returns {Promise<Object>}
     */
    async saveFeatures(publisherId, siteId, unitId, data) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/sites/${encodeURIComponent(siteId)}/ad-units/${encodeURIComponent(unitId)}/features`,
            {
                method: 'PUT',
                body: JSON.stringify(data),
            }
        );
    },
};

/**
 * Publisher Bidders API
 * Manages bidder configuration at publisher level with multi-instance support
 */
export const publisherBidders = {
    /**
     * List all bidders for a publisher (global + publisher-specific)
     * @param {string} publisherId - Publisher ID
     * @returns {Promise<{bidders: Array, enabled: Array}>}
     */
    async list(publisherId) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders`);
    },

    /**
     * Get a single bidder configuration
     * @param {string} publisherId - Publisher ID
     * @param {string} bidderCode - Bidder code (e.g., "appnexus" or "appnexus-2")
     * @returns {Promise<Object>}
     */
    async get(publisherId, bidderCode) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders/${encodeURIComponent(bidderCode)}`
        );
    },

    /**
     * Create a new publisher-specific bidder instance
     * @param {string} publisherId - Publisher ID
     * @param {Object} data - Bidder configuration data
     * @returns {Promise<Object>}
     */
    async create(publisherId, data) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders`, {
            method: 'POST',
            body: JSON.stringify(data),
        });
    },

    /**
     * Update an existing bidder configuration
     * @param {string} publisherId - Publisher ID
     * @param {string} bidderCode - Bidder code
     * @param {Object} data - Updated bidder configuration
     * @returns {Promise<Object>}
     */
    async update(publisherId, bidderCode, data) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders/${encodeURIComponent(bidderCode)}`,
            {
                method: 'PUT',
                body: JSON.stringify(data),
            }
        );
    },

    /**
     * Delete a publisher-specific bidder instance
     * @param {string} publisherId - Publisher ID
     * @param {string} bidderCode - Bidder code
     * @returns {Promise<Object>}
     */
    async delete(publisherId, bidderCode) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders/${encodeURIComponent(bidderCode)}`,
            {
                method: 'DELETE',
            }
        );
    },

    /**
     * Enable a bidder for a publisher
     * @param {string} publisherId - Publisher ID
     * @param {string} bidderCode - Bidder code
     * @returns {Promise<Object>}
     */
    async enable(publisherId, bidderCode) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders/${encodeURIComponent(bidderCode)}/enable`,
            {
                method: 'POST',
            }
        );
    },

    /**
     * Disable a bidder for a publisher
     * @param {string} publisherId - Publisher ID
     * @param {string} bidderCode - Bidder code
     * @returns {Promise<Object>}
     */
    async disable(publisherId, bidderCode) {
        return request(
            `${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders/${encodeURIComponent(bidderCode)}/disable`,
            {
                method: 'POST',
            }
        );
    },

    /**
     * Enable or disable a bidder (convenience wrapper)
     * @param {string} publisherId - Publisher ID
     * @param {string} bidderCode - Bidder code
     * @param {boolean} enabled - Whether to enable or disable
     * @returns {Promise<Object>}
     */
    async setEnabled(publisherId, bidderCode, enabled) {
        return enabled
            ? this.enable(publisherId, bidderCode)
            : this.disable(publisherId, bidderCode);
    },

    /**
     * Duplicate a bidder with auto-suffix naming
     * @param {string} publisherId - Publisher ID
     * @param {string} sourceBidderCode - Source bidder code to duplicate
     * @param {string} name - Optional display name for the new instance
     * @returns {Promise<Object>}
     */
    async duplicate(publisherId, sourceBidderCode, name = null) {
        const data = { source_bidder_code: sourceBidderCode };
        if (name) {
            data.name = name;
        }
        return request(`${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders/duplicate`, {
            method: 'POST',
            body: JSON.stringify(data),
        });
    },

    /**
     * Get bidders grouped by family for UI display
     * @param {string} publisherId - Publisher ID
     * @returns {Promise<{families: Object}>}
     */
    async getFamilies(publisherId) {
        return request(`${BASE_URL}/publishers/${encodeURIComponent(publisherId)}/bidders/families`);
    },
};

/**
 * Features API helper - unified interface for features at any level
 */
export const features = {
    /**
     * Get features at any level
     * @param {string} publisherId - Publisher ID
     * @param {string|null} siteId - Site ID (optional)
     * @param {string|null} unitId - Ad unit ID (optional)
     * @returns {Promise<Object>}
     */
    async get(publisherId, siteId = null, unitId = null) {
        if (unitId) {
            return adUnits.getFeatures(publisherId, siteId, unitId);
        } else if (siteId) {
            return sites.getFeatures(publisherId, siteId);
        } else {
            return publishers.getFeatures(publisherId);
        }
    },

    /**
     * Save features at any level
     * @param {string} publisherId - Publisher ID
     * @param {string|null} siteId - Site ID (optional)
     * @param {string|null} unitId - Ad unit ID (optional)
     * @param {Object} data - Features data
     * @returns {Promise<Object>}
     */
    async save(publisherId, siteId = null, unitId = null, data) {
        if (unitId) {
            return adUnits.saveFeatures(publisherId, siteId, unitId, data);
        } else if (siteId) {
            return sites.saveFeatures(publisherId, siteId, data);
        } else {
            return publishers.saveFeatures(publisherId, data);
        }
    },
};

/**
 * Default export with all APIs
 */
export const API = {
    publishers,
    sites,
    adUnits,
    features,
    publisherBidders,
};

export default API;
