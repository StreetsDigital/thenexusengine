/**
 * Publishers Page - Taxonomy Management
 * Handles the tree view and CRUD operations for Publishers, Sites, and Ad Units
 */

import { API } from './api.js';

// =================================================================
// State Management
// =================================================================
// NOTE: For future publisher-specific access, the state can be
// initialized with a single publisher context from the server
const state = {
    publishers: {},
    expandedNodes: new Set(),
    loading: false,
    editingPublisher: null,
    editingSite: null,
    editingUnit: null,
};

// =================================================================
// Utility Functions
// =================================================================
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text || '';
    return div.innerHTML;
}

function escapeAttr(text) {
    return (text || '').replace(/'/g, "\\'").replace(/"/g, '&quot;');
}

function showAlert(message, type) {
    const alert = document.getElementById('alert');
    alert.textContent = message;
    alert.className = `alert ${type} show`;
    setTimeout(() => alert.classList.remove('show'), 5000);
}

// =================================================================
// Data Loading (Lazy Load)
// =================================================================
async function loadPublishers() {
    try {
        state.loading = true;
        const data = await API.publishers.list();

        state.publishers = {};
        if (data.publishers) {
            for (const pub of data.publishers) {
                state.publishers[pub.publisher_id] = {
                    ...pub,
                    _loaded: false,
                };
            }
        }

        renderTree();
    } catch (error) {
        console.error('Failed to load publishers:', error);
        showAlert('Failed to load publishers: ' + error.message, 'error');
    } finally {
        state.loading = false;
    }
}

async function loadPublisherDetails(publisherId) {
    try {
        const data = await API.publishers.get(publisherId);
        state.publishers[publisherId] = {
            ...data,
            _loaded: true,
        };
        renderTree();
    } catch (error) {
        console.error('Failed to load publisher details:', error);
        showAlert('Failed to load publisher: ' + error.message, 'error');
    }
}

function reloadData() {
    state.publishers = {};
    state.expandedNodes.clear();
    loadPublishers();
}

// =================================================================
// Tree Rendering
// =================================================================
function renderTree() {
    const container = document.getElementById('tree-container');
    const publishers = Object.values(state.publishers);

    if (publishers.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">&#128203;</div>
                <p>No publishers configured yet</p>
                <p style="margin-top: 0.5rem;">
                    <button class="btn btn-primary" onclick="window.publishersUI.openPublisherModal()">
                        Add Your First Publisher
                    </button>
                </p>
            </div>
        `;
        return;
    }

    container.innerHTML = publishers.map(pub => renderPublisherNode(pub)).join('');
}

function renderPublisherNode(pub) {
    const isExpanded = state.expandedNodes.has(`pub:${pub.publisher_id}`);
    const siteCount = pub.sites ? pub.sites.length : (pub.site_count || 0);
    const unitCount = pub.sites
        ? pub.sites.reduce((sum, s) => sum + (s.ad_units?.length || 0), 0)
        : (pub.unit_count || 0);

    let childrenHtml = '';
    if (isExpanded && pub._loaded && pub.sites) {
        childrenHtml = `
            <div class="tree-children expanded">
                ${pub.sites.map(site => renderSiteNode(site, pub.publisher_id)).join('')}
            </div>
        `;
    } else if (isExpanded && !pub._loaded) {
        childrenHtml = `
            <div class="tree-children expanded">
                <div class="tree-loading">Loading...</div>
            </div>
        `;
    }

    const pubId = escapeAttr(pub.publisher_id);
    return `
        <div class="tree-node level-publisher" data-type="publisher" data-publisher-id="${pubId}">
            <div class="tree-node-header" onclick="window.publishersUI.togglePublisher('${pubId}')">
                <span class="tree-toggle ${isExpanded ? 'expanded' : ''} ${siteCount === 0 ? 'leaf' : ''}">&#9654;</span>
                <span class="tree-icon">&#127970;</span>
                <div class="node-content">
                    <span class="node-name">${escapeHtml(pub.name || pub.publisher_id)}</span>
                    <span class="node-id">${escapeHtml(pub.publisher_id)}</span>
                    <span class="node-meta">
                        ${siteCount} site${siteCount !== 1 ? 's' : ''}, ${unitCount} unit${unitCount !== 1 ? 's' : ''}
                    </span>
                    <span class="status-badge ${pub.enabled !== false ? 'status-enabled' : 'status-disabled'}">
                        ${pub.enabled !== false ? 'enabled' : 'disabled'}
                    </span>
                </div>
                <div class="node-actions" onclick="event.stopPropagation()">
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.openFeaturesModal('publisher', '${pubId}')">Configure</button>
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.openSiteModal('${pubId}')">+ Site</button>
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.editPublisher('${pubId}')">Edit</button>
                    <button class="btn btn-xs btn-danger" onclick="window.publishersUI.confirmDeletePublisher('${pubId}')">Delete</button>
                </div>
            </div>
            ${childrenHtml}
        </div>
    `;
}

function renderSiteNode(site, publisherId) {
    const isExpanded = state.expandedNodes.has(`site:${publisherId}:${site.site_id}`);
    const unitCount = site.ad_units ? site.ad_units.length : 0;
    const hasOverrides = site.features && Object.keys(site.features).length > 0;

    let childrenHtml = '';
    if (isExpanded && site.ad_units) {
        childrenHtml = `
            <div class="tree-children expanded">
                ${site.ad_units.map(unit => renderUnitNode(unit, publisherId, site.site_id)).join('')}
            </div>
        `;
    }

    const pubId = escapeAttr(publisherId);
    const siteId = escapeAttr(site.site_id);
    return `
        <div class="tree-node level-site" data-type="site" data-publisher-id="${pubId}" data-site-id="${siteId}">
            <div class="tree-node-header" onclick="window.publishersUI.toggleSite('${pubId}', '${siteId}')">
                <span class="tree-toggle ${isExpanded ? 'expanded' : ''} ${unitCount === 0 ? 'leaf' : ''}">&#9654;</span>
                <span class="tree-icon">&#127760;</span>
                <div class="node-content">
                    <span class="node-name">${escapeHtml(site.name || site.site_id)}</span>
                    <span class="node-id">${escapeHtml(site.domain || site.site_id)}</span>
                    <span class="node-meta">${unitCount} unit${unitCount !== 1 ? 's' : ''}</span>
                    <span class="status-badge ${site.enabled !== false ? 'status-enabled' : 'status-disabled'}">
                        ${site.enabled !== false ? 'enabled' : 'disabled'}
                    </span>
                    ${hasOverrides ? '<span class="override-badge">overrides</span>' : ''}
                </div>
                <div class="node-actions" onclick="event.stopPropagation()">
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.openFeaturesModal('site', '${pubId}', '${siteId}')">Configure</button>
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.openUnitModal('${pubId}', '${siteId}')">+ Unit</button>
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.editSite('${pubId}', '${siteId}')">Edit</button>
                    <button class="btn btn-xs btn-danger" onclick="window.publishersUI.confirmDeleteSite('${pubId}', '${siteId}')">Delete</button>
                </div>
            </div>
            ${childrenHtml}
        </div>
    `;
}

function renderUnitNode(unit, publisherId, siteId) {
    const hasOverrides = unit.features && Object.keys(unit.features).length > 0;
    const sizeStr = unit.sizes ? unit.sizes.map(s => `${s[0]}x${s[1]}`).join(', ') : '';

    const pubId = escapeAttr(publisherId);
    const sId = escapeAttr(siteId);
    const unitId = escapeAttr(unit.unit_id);
    return `
        <div class="tree-node level-unit" data-type="unit" data-publisher-id="${pubId}" data-site-id="${sId}" data-unit-id="${unitId}">
            <div class="tree-node-header">
                <span class="tree-toggle leaf">&#9654;</span>
                <span class="tree-icon">&#128230;</span>
                <div class="node-content">
                    <span class="node-name">${escapeHtml(unit.name || unit.unit_id)}</span>
                    <span class="type-badge">${escapeHtml(unit.media_type || 'banner')}</span>
                    ${unit.floor_price ? `<span class="node-meta">$${unit.floor_price.toFixed(2)}</span>` : ''}
                    ${sizeStr ? `<span class="node-meta" title="${escapeAttr(sizeStr)}">${sizeStr.length > 30 ? sizeStr.substring(0, 30) + '...' : sizeStr}</span>` : ''}
                    ${hasOverrides ? '<span class="override-badge">overrides</span>' : ''}
                </div>
                <div class="node-actions" onclick="event.stopPropagation()">
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.openFeaturesModal('unit', '${pubId}', '${sId}', '${unitId}')">Configure</button>
                    <button class="btn btn-xs btn-secondary" onclick="window.publishersUI.editUnit('${pubId}', '${sId}', '${unitId}')">Edit</button>
                    <button class="btn btn-xs btn-danger" onclick="window.publishersUI.confirmDeleteUnit('${pubId}', '${sId}', '${unitId}')">Delete</button>
                </div>
            </div>
        </div>
    `;
}

// =================================================================
// Tree Interaction
// =================================================================
async function togglePublisher(publisherId) {
    const key = `pub:${publisherId}`;
    if (state.expandedNodes.has(key)) {
        state.expandedNodes.delete(key);
    } else {
        state.expandedNodes.add(key);
        if (!state.publishers[publisherId]._loaded) {
            await loadPublisherDetails(publisherId);
            return;
        }
    }
    renderTree();
}

function toggleSite(publisherId, siteId) {
    const key = `site:${publisherId}:${siteId}`;
    if (state.expandedNodes.has(key)) {
        state.expandedNodes.delete(key);
    } else {
        state.expandedNodes.add(key);
    }
    renderTree();
}

function expandAll() {
    for (const pubId of Object.keys(state.publishers)) {
        state.expandedNodes.add(`pub:${pubId}`);
        const pub = state.publishers[pubId];
        if (pub.sites) {
            for (const site of pub.sites) {
                state.expandedNodes.add(`site:${pubId}:${site.site_id}`);
            }
        }
    }
    Promise.all(
        Object.keys(state.publishers)
            .filter(id => !state.publishers[id]._loaded)
            .map(id => loadPublisherDetails(id))
    );
    renderTree();
}

function collapseAll() {
    state.expandedNodes.clear();
    renderTree();
}

// =================================================================
// Publisher CRUD
// =================================================================
function openPublisherModal(publisherId = null) {
    state.editingPublisher = publisherId;
    const modal = document.getElementById('publisher-modal');
    const title = document.getElementById('publisher-modal-title');
    const saveBtn = document.getElementById('pub-save-btn');
    const idInput = document.getElementById('pub-id');

    document.getElementById('publisher-form').reset();

    if (publisherId) {
        const pub = state.publishers[publisherId];
        title.textContent = 'Edit Publisher';
        saveBtn.textContent = 'Save Changes';
        idInput.value = publisherId;
        idInput.disabled = true;
        document.getElementById('pub-name').value = pub.name || '';
        document.getElementById('pub-enabled').checked = pub.enabled !== false;
        document.getElementById('pub-contact-name').value = pub.contact?.name || '';
        document.getElementById('pub-contact-email').value = pub.contact?.email || '';
    } else {
        title.textContent = 'Add Publisher';
        saveBtn.textContent = 'Create Publisher';
        idInput.disabled = false;
    }

    modal.classList.add('show');
}

function closePublisherModal() {
    document.getElementById('publisher-modal').classList.remove('show');
    state.editingPublisher = null;
}

function editPublisher(publisherId) {
    openPublisherModal(publisherId);
}

async function savePublisher(event) {
    event.preventDefault();

    const publisherId = document.getElementById('pub-id').value;
    const data = {
        publisher_id: publisherId,
        name: document.getElementById('pub-name').value,
        enabled: document.getElementById('pub-enabled').checked,
        contact: {
            name: document.getElementById('pub-contact-name').value,
            email: document.getElementById('pub-contact-email').value,
        },
    };

    try {
        await API.publishers.save(publisherId, data);
        showAlert(state.editingPublisher ? 'Publisher updated' : 'Publisher created', 'success');
        closePublisherModal();
        await loadPublishers();
    } catch (error) {
        showAlert(error.message, 'error');
    }
}

// =================================================================
// Site CRUD
// =================================================================
function openSiteModal(publisherId, siteId = null) {
    state.editingSite = siteId;
    document.getElementById('site-publisher-id').value = publisherId;

    const modal = document.getElementById('site-modal');
    const title = document.getElementById('site-modal-title');
    const saveBtn = document.getElementById('site-save-btn');
    const idInput = document.getElementById('site-id');

    document.getElementById('site-form').reset();

    if (siteId) {
        const pub = state.publishers[publisherId];
        const site = pub.sites?.find(s => s.site_id === siteId);
        if (site) {
            title.textContent = 'Edit Site';
            saveBtn.textContent = 'Save Changes';
            idInput.value = siteId;
            idInput.disabled = true;
            document.getElementById('site-domain').value = site.domain || '';
            document.getElementById('site-name').value = site.name || '';
            document.getElementById('site-enabled').checked = site.enabled !== false;
        }
    } else {
        title.textContent = 'Add Site';
        saveBtn.textContent = 'Create Site';
        idInput.disabled = false;
    }

    modal.classList.add('show');
}

function closeSiteModal() {
    document.getElementById('site-modal').classList.remove('show');
    state.editingSite = null;
}

function editSite(publisherId, siteId) {
    openSiteModal(publisherId, siteId);
}

async function saveSite(event) {
    event.preventDefault();

    const publisherId = document.getElementById('site-publisher-id').value;
    const siteId = document.getElementById('site-id').value;
    const data = {
        site_id: siteId,
        domain: document.getElementById('site-domain').value,
        name: document.getElementById('site-name').value,
        enabled: document.getElementById('site-enabled').checked,
    };

    try {
        await API.sites.save(publisherId, siteId, data);
        showAlert(state.editingSite ? 'Site updated' : 'Site created', 'success');
        closeSiteModal();
        await loadPublisherDetails(publisherId);
    } catch (error) {
        showAlert(error.message, 'error');
    }
}

// =================================================================
// Ad Unit CRUD
// =================================================================
function openUnitModal(publisherId, siteId, unitId = null) {
    state.editingUnit = unitId;
    document.getElementById('unit-publisher-id').value = publisherId;
    document.getElementById('unit-site-id').value = siteId;

    const modal = document.getElementById('unit-modal');
    const title = document.getElementById('unit-modal-title');
    const saveBtn = document.getElementById('unit-save-btn');
    const idInput = document.getElementById('unit-id');

    document.getElementById('unit-form').reset();
    resetSizeRows();

    if (unitId) {
        const pub = state.publishers[publisherId];
        const site = pub.sites?.find(s => s.site_id === siteId);
        const unit = site?.ad_units?.find(u => u.unit_id === unitId);
        if (unit) {
            title.textContent = 'Edit Ad Unit';
            saveBtn.textContent = 'Save Changes';
            idInput.value = unitId;
            idInput.disabled = true;
            document.getElementById('unit-name').value = unit.name || '';
            document.getElementById('unit-media-type').value = unit.media_type || 'banner';
            document.getElementById('unit-position').value = unit.position || 'unknown';
            document.getElementById('unit-floor').value = unit.floor_price || '';
            document.getElementById('unit-currency').value = unit.floor_currency || 'USD';

            if (unit.sizes && unit.sizes.length > 0) {
                setSizeRows(unit.sizes);
            }
        }
    } else {
        title.textContent = 'Add Ad Unit';
        saveBtn.textContent = 'Create Ad Unit';
        idInput.disabled = false;
    }

    modal.classList.add('show');
}

function closeUnitModal() {
    document.getElementById('unit-modal').classList.remove('show');
    state.editingUnit = null;
}

function editUnit(publisherId, siteId, unitId) {
    openUnitModal(publisherId, siteId, unitId);
}

function resetSizeRows() {
    const container = document.getElementById('sizes-container');
    container.innerHTML = `
        <div class="size-row">
            <input type="number" class="size-width" placeholder="Width" min="1">
            <span class="size-x">x</span>
            <input type="number" class="size-height" placeholder="Height" min="1">
            <button type="button" class="btn btn-xs btn-secondary" onclick="window.publishersUI.addSizeRow()">+</button>
        </div>
    `;
}

function setSizeRows(sizes) {
    const container = document.getElementById('sizes-container');
    container.innerHTML = sizes.map((size, i) => `
        <div class="size-row">
            <input type="number" class="size-width" placeholder="Width" min="1" value="${size[0]}">
            <span class="size-x">x</span>
            <input type="number" class="size-height" placeholder="Height" min="1" value="${size[1]}">
            ${i === 0
                ? '<button type="button" class="btn btn-xs btn-secondary" onclick="window.publishersUI.addSizeRow()">+</button>'
                : '<button type="button" class="btn btn-xs btn-danger" onclick="window.publishersUI.removeSizeRow(this)">-</button>'
            }
        </div>
    `).join('');
}

function addSizeRow() {
    const container = document.getElementById('sizes-container');
    const row = document.createElement('div');
    row.className = 'size-row';
    row.innerHTML = `
        <input type="number" class="size-width" placeholder="Width" min="1">
        <span class="size-x">x</span>
        <input type="number" class="size-height" placeholder="Height" min="1">
        <button type="button" class="btn btn-xs btn-danger" onclick="window.publishersUI.removeSizeRow(this)">-</button>
    `;
    container.appendChild(row);
}

function removeSizeRow(btn) {
    btn.closest('.size-row').remove();
}

function getSizes() {
    const sizes = [];
    document.querySelectorAll('.size-row').forEach(row => {
        const w = parseInt(row.querySelector('.size-width').value);
        const h = parseInt(row.querySelector('.size-height').value);
        if (w > 0 && h > 0) {
            sizes.push([w, h]);
        }
    });
    return sizes;
}

async function saveUnit(event) {
    event.preventDefault();

    const publisherId = document.getElementById('unit-publisher-id').value;
    const siteId = document.getElementById('unit-site-id').value;
    const unitId = document.getElementById('unit-id').value;

    const sizes = getSizes();
    const data = {
        unit_id: unitId,
        name: document.getElementById('unit-name').value,
        media_type: document.getElementById('unit-media-type').value,
        position: document.getElementById('unit-position').value,
        sizes: sizes,
        floor_price: parseFloat(document.getElementById('unit-floor').value) || null,
        floor_currency: document.getElementById('unit-currency').value,
    };

    try {
        await API.adUnits.save(publisherId, siteId, unitId, data);
        showAlert(state.editingUnit ? 'Ad unit updated' : 'Ad unit created', 'success');
        closeUnitModal();
        await loadPublisherDetails(publisherId);
    } catch (error) {
        showAlert(error.message, 'error');
    }
}

// =================================================================
// Features Modal
// =================================================================
async function openFeaturesModal(type, publisherId, siteId = null, unitId = null) {
    document.getElementById('features-type').value = type;
    document.getElementById('features-publisher-id').value = publisherId;
    document.getElementById('features-site-id').value = siteId || '';
    document.getElementById('features-unit-id').value = unitId || '';

    const title = document.getElementById('features-modal-title');
    if (type === 'publisher') {
        title.textContent = `Configure Publisher: ${publisherId}`;
    } else if (type === 'site') {
        title.textContent = `Configure Site: ${siteId}`;
    } else {
        title.textContent = `Configure Ad Unit: ${unitId}`;
    }

    document.getElementById('features-form').reset();

    try {
        const data = await API.features.get(publisherId, siteId, unitId);
        populateFeaturesForm(data);
    } catch (error) {
        console.log('No existing features, using defaults');
    }

    document.getElementById('features-modal').classList.add('show');
}

function closeFeaturesModal() {
    document.getElementById('features-modal').classList.remove('show');
}

function populateFeaturesForm(data) {
    // IDR
    if (data.idr) {
        document.getElementById('feat-max-bidders').value = data.idr.max_bidders || '';
        document.getElementById('feat-min-score').value = data.idr.min_score_threshold || '';
        document.getElementById('feat-exploration-rate').value = data.idr.exploration_rate || '';
        document.getElementById('feat-timeout').value = data.idr.selection_timeout_ms || '';
        document.getElementById('feat-anchor-bidders').value = (data.idr.custom_anchor_bidders || []).join(', ');
    }

    // Floors
    if (data.floors) {
        document.getElementById('feat-default-floor').value = data.floors.default_floor_price || '';
        document.getElementById('feat-floor-currency').value = data.floors.floor_currency || 'USD';
        document.getElementById('feat-banner-floor').value = data.floors.banner_floor || '';
        document.getElementById('feat-video-floor').value = data.floors.video_floor || '';
        document.getElementById('feat-native-floor').value = data.floors.native_floor || '';
        document.getElementById('feat-dynamic-floors').checked = data.floors.dynamic_floors_enabled || false;
    }

    // Bidders
    if (data.bidders) {
        document.getElementById('feat-enabled-bidders').value = (data.bidders.enabled_bidders || []).join('\n');
        document.getElementById('feat-disabled-bidders').value = (data.bidders.disabled_bidders || []).join('\n');
    }

    // Privacy
    if (data.privacy) {
        document.getElementById('feat-gdpr').checked = data.privacy.gdpr_applies || false;
        document.getElementById('feat-ccpa').checked = data.privacy.ccpa_applies || false;
        document.getElementById('feat-coppa').checked = data.privacy.coppa_applies || false;
        document.getElementById('feat-strict-privacy').checked = data.privacy.privacy_strict_mode || false;
    }
}

async function saveFeatures(event) {
    event.preventDefault();

    const publisherId = document.getElementById('features-publisher-id').value;
    const siteId = document.getElementById('features-site-id').value || null;
    const unitId = document.getElementById('features-unit-id').value || null;

    const data = {
        idr: {
            max_bidders: parseInt(document.getElementById('feat-max-bidders').value) || null,
            min_score_threshold: parseFloat(document.getElementById('feat-min-score').value) || null,
            exploration_rate: parseFloat(document.getElementById('feat-exploration-rate').value) || null,
            selection_timeout_ms: parseInt(document.getElementById('feat-timeout').value) || null,
            custom_anchor_bidders: document.getElementById('feat-anchor-bidders').value
                .split(',')
                .map(s => s.trim())
                .filter(s => s),
        },
        floors: {
            default_floor_price: parseFloat(document.getElementById('feat-default-floor').value) || null,
            floor_currency: document.getElementById('feat-floor-currency').value,
            banner_floor: parseFloat(document.getElementById('feat-banner-floor').value) || null,
            video_floor: parseFloat(document.getElementById('feat-video-floor').value) || null,
            native_floor: parseFloat(document.getElementById('feat-native-floor').value) || null,
            dynamic_floors_enabled: document.getElementById('feat-dynamic-floors').checked,
        },
        bidders: {
            enabled_bidders: document.getElementById('feat-enabled-bidders').value
                .split('\n')
                .map(s => s.trim())
                .filter(s => s),
            disabled_bidders: document.getElementById('feat-disabled-bidders').value
                .split('\n')
                .map(s => s.trim())
                .filter(s => s),
        },
        privacy: {
            gdpr_applies: document.getElementById('feat-gdpr').checked,
            ccpa_applies: document.getElementById('feat-ccpa').checked,
            coppa_applies: document.getElementById('feat-coppa').checked,
            privacy_strict_mode: document.getElementById('feat-strict-privacy').checked,
        },
    };

    try {
        await API.features.save(publisherId, siteId, unitId, data);
        showAlert('Features saved', 'success');
        closeFeaturesModal();
        await loadPublisherDetails(publisherId);
    } catch (error) {
        showAlert(error.message, 'error');
    }
}

// =================================================================
// Bidder Management
// =================================================================
const bidderState = {
    families: {},
    searchFilter: '',
    currentPublisherId: null,
    editingBidderCode: null,
};

async function loadBidders(publisherId) {
    bidderState.currentPublisherId = publisherId;
    const container = document.getElementById('bidders-list');
    container.innerHTML = '<div class="loading-bidders">Loading bidders...</div>';

    try {
        const data = await API.publisherBidders.getFamilies(publisherId);
        bidderState.families = data.families || {};
        renderBiddersList();
    } catch (error) {
        console.error('Failed to load bidders:', error);
        container.innerHTML = '<div class="bidders-error">Failed to load bidders</div>';
    }
}

function renderBiddersList() {
    const container = document.getElementById('bidders-list');
    const families = bidderState.families;
    const filter = bidderState.searchFilter.toLowerCase();

    const familyNames = Object.keys(families).sort();

    if (familyNames.length === 0) {
        container.innerHTML = `
            <div class="bidders-empty">
                <p>No bidders configured yet</p>
                <button class="btn btn-primary" onclick="publishersUI.openAddBidderModal()">Add Your First Bidder</button>
            </div>
        `;
        return;
    }

    let html = '';
    for (const family of familyNames) {
        const bidders = families[family];
        const filteredBidders = filter
            ? bidders.filter(b =>
                b.name?.toLowerCase().includes(filter) ||
                b.bidder_code?.toLowerCase().includes(filter) ||
                family.toLowerCase().includes(filter))
            : bidders;

        if (filteredBidders.length === 0) continue;

        html += `
            <div class="bidder-family">
                <div class="bidder-family-header" onclick="publishersUI.toggleBidderFamily('${escapeAttr(family)}')">
                    <span class="family-toggle" id="toggle-${escapeAttr(family)}">▼</span>
                    <span class="family-name">${escapeHtml(family)}</span>
                    <span class="family-count">(${filteredBidders.length} instance${filteredBidders.length !== 1 ? 's' : ''})</span>
                </div>
                <div class="bidder-family-instances" id="family-${escapeAttr(family)}">
                    ${filteredBidders.map(b => renderBidderRow(b)).join('')}
                </div>
            </div>
        `;
    }

    container.innerHTML = html || '<div class="bidders-empty">No bidders match your search</div>';
}

function renderBidderRow(bidder) {
    const isEnabled = bidder.is_enabled === true;
    const isCustom = bidder.is_global === false;
    const code = escapeAttr(bidder.bidder_code);

    return `
        <div class="bidder-row ${isEnabled ? 'enabled' : 'disabled'}">
            <div class="bidder-toggle">
                <label class="toggle-switch">
                    <input type="checkbox" ${isEnabled ? 'checked' : ''}
                           onchange="publishersUI.toggleBidderEnabled('${code}', this.checked)">
                    <span class="toggle-slider"></span>
                </label>
            </div>
            <div class="bidder-info">
                <span class="bidder-name">${escapeHtml(bidder.name || bidder.bidder_code)}</span>
                <span class="bidder-code">${escapeHtml(bidder.bidder_code)}</span>
                <span class="bidder-badge ${isCustom ? 'custom' : 'global'}">${isCustom ? 'custom' : 'global'}</span>
                <span class="bidder-status status-${bidder.status || 'active'}">${bidder.status || 'active'}</span>
            </div>
            <div class="bidder-actions">
                <button class="btn btn-xs btn-secondary" onclick="publishersUI.openBidderConfigModal('${code}')">Configure</button>
                <button class="btn btn-xs btn-secondary" onclick="publishersUI.duplicateBidder('${code}')">Duplicate</button>
                ${isCustom ? `<button class="btn btn-xs btn-danger" onclick="publishersUI.confirmDeleteBidder('${code}')">Delete</button>` : ''}
            </div>
        </div>
    `;
}

function toggleBidderFamily(family) {
    const container = document.getElementById(`family-${family}`);
    const toggle = document.getElementById(`toggle-${family}`);
    if (container.classList.contains('collapsed')) {
        container.classList.remove('collapsed');
        toggle.textContent = '▼';
    } else {
        container.classList.add('collapsed');
        toggle.textContent = '►';
    }
}

async function toggleBidderEnabled(bidderCode, enabled) {
    try {
        await API.publisherBidders.setEnabled(bidderState.currentPublisherId, bidderCode, enabled);
        // Update local state for immediate UI feedback
        for (const family of Object.values(bidderState.families)) {
            for (const bidder of family) {
                if (bidder.bidder_code === bidderCode) {
                    bidder.is_enabled = enabled;
                    break;
                }
            }
        }
        showAlert(`Bidder ${bidderCode} ${enabled ? 'enabled' : 'disabled'}`, 'success');
        renderBiddersList();
    } catch (error) {
        showAlert(error.message, 'error');
        await loadBidders(bidderState.currentPublisherId); // Reload to get correct state
    }
}

function refreshBidders() {
    if (bidderState.currentPublisherId) {
        loadBidders(bidderState.currentPublisherId);
    }
}

function filterBidders() {
    bidderState.searchFilter = document.getElementById('bidder-search').value;
    renderBiddersList();
}

// Bidder Configuration Modal
function openAddBidderModal() {
    bidderState.editingBidderCode = null;
    const modal = document.getElementById('bidder-modal');
    const title = document.getElementById('bidder-modal-title');
    const saveBtn = document.getElementById('bidder-save-btn');

    document.getElementById('bidder-form').reset();
    document.getElementById('bidder-publisher-id').value = bidderState.currentPublisherId;
    document.getElementById('bidder-original-code').value = '';
    document.getElementById('bidder-is-new').value = 'true';

    title.textContent = 'Add New Bidder';
    saveBtn.textContent = 'Create Bidder';

    // Reset collapsible sections
    document.querySelectorAll('.config-section').forEach((section, i) => {
        if (i === 0) {
            section.classList.add('expanded');
            section.querySelector('.section-toggle').textContent = '▼';
        } else {
            section.classList.remove('expanded');
            section.querySelector('.section-toggle').textContent = '►';
        }
    });

    modal.classList.add('show');
}

async function openBidderConfigModal(bidderCode) {
    bidderState.editingBidderCode = bidderCode;
    const modal = document.getElementById('bidder-modal');
    const title = document.getElementById('bidder-modal-title');
    const saveBtn = document.getElementById('bidder-save-btn');

    document.getElementById('bidder-form').reset();
    document.getElementById('bidder-publisher-id').value = bidderState.currentPublisherId;
    document.getElementById('bidder-original-code').value = bidderCode;
    document.getElementById('bidder-is-new').value = 'false';

    title.textContent = `Configure Bidder: ${bidderCode}`;
    saveBtn.textContent = 'Save Changes';

    try {
        const data = await API.publisherBidders.get(bidderState.currentPublisherId, bidderCode);
        populateBidderForm(data.bidder || data);
    } catch (error) {
        showAlert('Failed to load bidder config: ' + error.message, 'error');
        return;
    }

    modal.classList.add('show');
}

function populateBidderForm(bidder) {
    // Basic config
    document.getElementById('bidder-name').value = bidder.name || '';
    document.getElementById('bidder-family').value = bidder.bidder_family || bidder.bidder_code || '';
    document.getElementById('bidder-code').value = bidder.bidder_code || '';
    document.getElementById('bidder-status').value = bidder.status || 'active';
    document.getElementById('bidder-timeout').value = bidder.timeout_ms || 200;
    document.getElementById('bidder-gvl-id').value = bidder.gvl_vendor_id || '';

    if (bidder.endpoint) {
        document.getElementById('bidder-endpoint').value = bidder.endpoint.url || '';
    }

    // Capabilities
    const caps = bidder.capabilities || {};
    document.getElementById('bidder-cap-banner').checked = caps.banner !== false;
    document.getElementById('bidder-cap-video').checked = caps.video || false;
    document.getElementById('bidder-cap-native').checked = caps.native || false;
    document.getElementById('bidder-cap-audio').checked = caps.audio || false;

    // Rate limits
    const limits = bidder.rate_limits || {};
    document.getElementById('bidder-qps').value = limits.max_qps || 100;
    document.getElementById('bidder-daily-limit').value = limits.daily_limit || '';
    document.getElementById('bidder-concurrent').value = limits.max_concurrent || 10;

    // Request transform
    const reqTransform = bidder.request_transform || {};
    document.getElementById('bidder-seat-id').value = reqTransform.seat_id || '';
    document.getElementById('bidder-imp-ext').value =
        reqTransform.imp_ext_template ? JSON.stringify(reqTransform.imp_ext_template, null, 2) : '';
    document.getElementById('bidder-req-ext').value =
        reqTransform.request_ext_template ? JSON.stringify(reqTransform.request_ext_template, null, 2) : '';

    // Response transform
    const resTransform = bidder.response_transform || {};
    document.getElementById('bidder-price-adj').value = resTransform.price_adjustment || 1.0;
    document.getElementById('bidder-currency').value = resTransform.currency_override || '';

    // Targeting
    const targeting = bidder.targeting || {};
    document.getElementById('bidder-countries').value = (targeting.allowed_countries || []).join(', ');

    // Privacy
    const gdpr = bidder.gdpr || {};
    document.getElementById('bidder-gdpr-enabled').checked = gdpr.enabled !== false;
    document.getElementById('bidder-ccpa-enabled').checked = gdpr.ccpa_enabled !== false;
    document.getElementById('bidder-require-consent').checked = gdpr.require_consent || false;
}

function closeBidderModal() {
    document.getElementById('bidder-modal').classList.remove('show');
    bidderState.editingBidderCode = null;
}

function toggleConfigSection(header) {
    const section = header.closest('.config-section');
    const toggle = section.querySelector('.section-toggle');
    if (section.classList.contains('expanded')) {
        section.classList.remove('expanded');
        toggle.textContent = '►';
    } else {
        section.classList.add('expanded');
        toggle.textContent = '▼';
    }
}

async function saveBidder(event) {
    event.preventDefault();

    const publisherId = document.getElementById('bidder-publisher-id').value;
    const isNew = document.getElementById('bidder-is-new').value === 'true';
    const originalCode = document.getElementById('bidder-original-code').value;

    // Parse JSON fields
    let impExt = {};
    let reqExt = {};
    try {
        const impExtStr = document.getElementById('bidder-imp-ext').value.trim();
        if (impExtStr) impExt = JSON.parse(impExtStr);
    } catch (e) {
        showAlert('Invalid JSON in Imp Ext Template', 'error');
        return;
    }
    try {
        const reqExtStr = document.getElementById('bidder-req-ext').value.trim();
        if (reqExtStr) reqExt = JSON.parse(reqExtStr);
    } catch (e) {
        showAlert('Invalid JSON in Request Ext Template', 'error');
        return;
    }

    const data = {
        name: document.getElementById('bidder-name').value,
        bidder_family: document.getElementById('bidder-family').value,
        status: document.getElementById('bidder-status').value,
        timeout_ms: parseInt(document.getElementById('bidder-timeout').value) || 200,
        gvl_vendor_id: parseInt(document.getElementById('bidder-gvl-id').value) || null,
        endpoint: {
            url: document.getElementById('bidder-endpoint').value || null,
        },
        capabilities: {
            banner: document.getElementById('bidder-cap-banner').checked,
            video: document.getElementById('bidder-cap-video').checked,
            native: document.getElementById('bidder-cap-native').checked,
            audio: document.getElementById('bidder-cap-audio').checked,
        },
        rate_limits: {
            max_qps: parseInt(document.getElementById('bidder-qps').value) || 100,
            daily_limit: parseInt(document.getElementById('bidder-daily-limit').value) || null,
            max_concurrent: parseInt(document.getElementById('bidder-concurrent').value) || 10,
        },
        request_transform: {
            seat_id: document.getElementById('bidder-seat-id').value || null,
            imp_ext_template: Object.keys(impExt).length ? impExt : null,
            request_ext_template: Object.keys(reqExt).length ? reqExt : null,
        },
        response_transform: {
            price_adjustment: parseFloat(document.getElementById('bidder-price-adj').value) || 1.0,
            currency_override: document.getElementById('bidder-currency').value || null,
        },
        targeting: {
            allowed_countries: document.getElementById('bidder-countries').value
                .split(',')
                .map(s => s.trim().toUpperCase())
                .filter(s => s),
        },
        gdpr: {
            enabled: document.getElementById('bidder-gdpr-enabled').checked,
            ccpa_enabled: document.getElementById('bidder-ccpa-enabled').checked,
            require_consent: document.getElementById('bidder-require-consent').checked,
        },
    };

    try {
        if (isNew) {
            await API.publisherBidders.create(publisherId, data);
            showAlert('Bidder created', 'success');
        } else {
            await API.publisherBidders.update(publisherId, originalCode, data);
            showAlert('Bidder updated', 'success');
        }
        closeBidderModal();
        await loadBidders(publisherId);
    } catch (error) {
        showAlert(error.message, 'error');
    }
}

async function duplicateBidder(bidderCode) {
    try {
        const result = await API.publisherBidders.duplicate(
            bidderState.currentPublisherId,
            bidderCode
        );
        showAlert(`Bidder duplicated as ${result.bidder_code}`, 'success');
        await loadBidders(bidderState.currentPublisherId);
    } catch (error) {
        showAlert(error.message, 'error');
    }
}

function confirmDeleteBidder(bidderCode) {
    document.getElementById('delete-message').textContent =
        `Are you sure you want to delete bidder "${bidderCode}"?`;
    deleteCallback = async () => {
        try {
            await API.publisherBidders.delete(bidderState.currentPublisherId, bidderCode);
            showAlert('Bidder deleted', 'success');
            await loadBidders(bidderState.currentPublisherId);
        } catch (error) {
            showAlert(error.message, 'error');
        }
    };
    document.getElementById('delete-modal').classList.add('show');
}

function testBidderEndpoint() {
    const endpoint = document.getElementById('bidder-endpoint').value;
    if (!endpoint) {
        showAlert('No endpoint URL specified', 'error');
        return;
    }
    showAlert('Endpoint testing not yet implemented', 'info');
}

// =================================================================
// Delete Confirmation
// =================================================================
let deleteCallback = null;

function confirmDeletePublisher(publisherId) {
    document.getElementById('delete-message').textContent =
        `Are you sure you want to delete publisher "${publisherId}" and all its sites and ad units?`;
    deleteCallback = async () => {
        try {
            await API.publishers.delete(publisherId);
            showAlert('Publisher deleted', 'success');
            delete state.publishers[publisherId];
            renderTree();
        } catch (error) {
            showAlert(error.message, 'error');
        }
    };
    document.getElementById('delete-modal').classList.add('show');
}

function confirmDeleteSite(publisherId, siteId) {
    document.getElementById('delete-message').textContent =
        `Are you sure you want to delete site "${siteId}" and all its ad units?`;
    deleteCallback = async () => {
        try {
            await API.sites.delete(publisherId, siteId);
            showAlert('Site deleted', 'success');
            await loadPublisherDetails(publisherId);
        } catch (error) {
            showAlert(error.message, 'error');
        }
    };
    document.getElementById('delete-modal').classList.add('show');
}

function confirmDeleteUnit(publisherId, siteId, unitId) {
    document.getElementById('delete-message').textContent =
        `Are you sure you want to delete ad unit "${unitId}"?`;
    deleteCallback = async () => {
        try {
            await API.adUnits.delete(publisherId, siteId, unitId);
            showAlert('Ad unit deleted', 'success');
            await loadPublisherDetails(publisherId);
        } catch (error) {
            showAlert(error.message, 'error');
        }
    };
    document.getElementById('delete-modal').classList.add('show');
}

function closeDeleteModal() {
    document.getElementById('delete-modal').classList.remove('show');
    deleteCallback = null;
}

async function executeDelete() {
    if (deleteCallback) {
        await deleteCallback();
        closeDeleteModal();
    }
}

// =================================================================
// Tabs
// =================================================================
function initTabs() {
    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
            const tabId = tab.dataset.tab;

            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

            tab.classList.add('active');
            document.getElementById(`tab-${tabId}`).classList.add('active');

            // Load bidders when Bidders tab is clicked
            if (tabId === 'bidders') {
                const publisherId = document.getElementById('features-publisher-id').value;
                if (publisherId) {
                    loadBidders(publisherId);
                }
            }
        });
    });

    // Bidder search filter
    const bidderSearch = document.getElementById('bidder-search');
    if (bidderSearch) {
        bidderSearch.addEventListener('input', filterBidders);
    }
}

// =================================================================
// Form Event Handlers
// =================================================================
function initFormHandlers() {
    document.getElementById('publisher-form').addEventListener('submit', savePublisher);
    document.getElementById('site-form').addEventListener('submit', saveSite);
    document.getElementById('unit-form').addEventListener('submit', saveUnit);
    document.getElementById('features-form').addEventListener('submit', saveFeatures);
    document.getElementById('delete-confirm-btn').addEventListener('click', executeDelete);

    // Bidder form handler
    const bidderForm = document.getElementById('bidder-form');
    if (bidderForm) {
        bidderForm.addEventListener('submit', saveBidder);
    }
}

// =================================================================
// Public API (exposed to window for onclick handlers)
// =================================================================
window.publishersUI = {
    // Tree
    togglePublisher,
    toggleSite,
    expandAll,
    collapseAll,
    reloadData,

    // Publisher
    openPublisherModal,
    closePublisherModal,
    editPublisher,
    confirmDeletePublisher,

    // Site
    openSiteModal,
    closeSiteModal,
    editSite,
    confirmDeleteSite,

    // Unit
    openUnitModal,
    closeUnitModal,
    editUnit,
    confirmDeleteUnit,
    addSizeRow,
    removeSizeRow,

    // Features
    openFeaturesModal,
    closeFeaturesModal,

    // Bidders
    refreshBidders,
    openAddBidderModal,
    openBidderConfigModal,
    closeBidderModal,
    toggleBidderFamily,
    toggleBidderEnabled,
    duplicateBidder,
    confirmDeleteBidder,
    toggleConfigSection,
    testBidderEndpoint,

    // Delete
    closeDeleteModal,
};

// =================================================================
// Initialize
// =================================================================
document.addEventListener('DOMContentLoaded', () => {
    initTabs();
    initFormHandlers();
    loadPublishers();
});
