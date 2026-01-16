// Global session expiry handler
function handleSessionExpiry() {
    console.log('[SESSION] Session expired - clearing caches and redirecting to login');

    // Clear all caches
    enrichedDataCache.clear();
    selectedItems.clear();
    listings = [];
    allListings = [];

    // Stop enrichment polling
    if (enrichmentPollInterval) {
        clearInterval(enrichmentPollInterval);
        enrichmentPollInterval = null;
    }

    // Reset auth state
    isAuthenticated = false;
    currentAccount = null;

    // Show alert to user
    alert('Your eBay session has expired. Please reconnect to continue.');

    // Reload page to show login screen
    window.location.href = '/';
}

// Fetch wrapper that handles session expiry
async function secureFetch(url, options = {}) {
    const response = await fetch(url, options);

    // Handle 401 Unauthorised - session expired
    if (response.status === 401) {
        handleSessionExpiry();
        throw new Error('Session expired');
    }

    return response;
}

// Theme handling
function initTheme() {
    const savedTheme = localStorage.getItem('theme');
    // Default to dark mode
    if (savedTheme === 'light') {
        document.body.classList.add('light-mode');
        document.getElementById('themeIcon').innerHTML = '&#9790;'; // Moon
    } else {
        document.getElementById('themeIcon').innerHTML = '&#9728;'; // Sun
    }
}

function toggleTheme() {
    const isLight = document.body.classList.toggle('light-mode');
    localStorage.setItem('theme', isLight ? 'light' : 'dark');
    document.getElementById('themeIcon').innerHTML = isLight ? '&#9790;' : '&#9728;';
}

// State
let isAuthenticated = false;
let isConfigured = false;
let currentAccountName = null;
let currentAccountEnv = null;
let listings = [];           // Current page/filtered view for display
let allListings = [];        // ALL listings from eBay (complete dataset)
let currentPage = 0;
let pageSize = parseInt(localStorage.getItem('pageSize') || '50', 10);
let totalListings = 0;
let selectedItems = new Set();
let searchFilter = '';
let sortColumn = '';
let sortDirection = 'asc';
let isLoadingAllListings = false;  // Progressive loading state
let listingsLoadProgress = 0;      // Number of listings loaded so far
let totalListingsFromAPI = 0;      // Total count from eBay API (for progressive loading)

// Carousel state
let carouselImages = new Map(); // offerId -> [imageUrls]
let currentCarouselImages = [];
let currentImageIndex = 0;
let brands = [];
let tariffCountries = [];
let weightBands = [];
let currentAccount = null;
let availableAccounts = [];

// Progressive enrichment state
let enrichmentPollInterval = null;
let enrichedDataCache = new Map(); // ItemID -> EnrichedItemData
let calculationCache = new Map();  // ItemID -> {calculatedCost, diff, diffStatus, expectedCoo, cooStatus}

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    initTheme();
    setupTabs();
    initPageSize();
    initSearchFilter();
    await checkAuthStatus();        // Check authentication status first
    await loadCurrentAccount();
    await loadReferenceData();

    // Check for auth success redirect
    const params = new URLSearchParams(window.location.search);
    if (params.get('auth') === 'success') {
        window.history.replaceState({}, '', '/');
        await checkAuthStatus();    // Refresh auth status after OAuth
        await loadCurrentAccount();  // Reload account after OAuth
        showTab('listings');
        loadListings();
    } else {
        // Since Listings is the default tab, load it automatically if authenticated
        if (isAuthenticated) {
            loadListings();
        }
    }
});

// Tab handling
function setupTabs() {
    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
            showTab(tab.dataset.tab);
        });
    });
}

// Page size handling
function initPageSize() {
    const select = document.getElementById('pageSizeSelect');
    if (select) {
        select.value = pageSize.toString();
    }
}

// Search filter handling - now searches ALL listings, not just current page
function initSearchFilter() {
    const searchInput = document.getElementById('listingsSearch');
    const clearBtn = document.getElementById('clearSearchBtn');
    if (searchInput) {
        // Use debounce for better performance
        let debounceTimer;
        searchInput.addEventListener('input', (e) => {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(() => {
                searchFilter = e.target.value.toLowerCase();
                currentPage = 0;  // Reset to first page when searching
                applyFiltersAndSort();
                renderListings();
                // Show/hide clear button
                if (clearBtn) {
                    clearBtn.style.display = searchFilter ? 'inline-block' : 'none';
                }
            }, 300);  // 300ms debounce
        });
    }
}

// Clear search filter
function clearSearch() {
    const searchInput = document.getElementById('listingsSearch');
    const clearBtn = document.getElementById('clearSearchBtn');
    if (searchInput) {
        searchInput.value = '';
        searchFilter = '';
        currentPage = 0;
        applyFiltersAndSort();
        renderListings();
    }
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
}

// Check authentication status
// Table sorting - now sorts ALL listings, not just current page
// 3-state toggle: asc → desc → reset (default ItemID desc)
function sortTable(column) {
    if (sortColumn === column) {
        // Same column: cycle through asc → desc → reset
        if (sortDirection === 'asc') {
            sortDirection = 'desc';
        } else if (sortDirection === 'desc') {
            // Reset to default (no active sort column)
            sortColumn = '';
            sortDirection = 'desc';
        }
    } else {
        // New column: start with ascending
        sortColumn = column;
        sortDirection = 'asc';
    }

    // Update header classes
    document.querySelectorAll('th.sortable').forEach(th => {
        th.classList.remove('sorted-asc', 'sorted-desc');
    });

    // Only show sort indicator if a column is actively sorted
    if (sortColumn) {
        const header = document.querySelector(`th[data-column="${column}"]`);
        if (header) {
            header.classList.add(sortDirection === 'asc' ? 'sorted-asc' : 'sorted-desc');
        }
    }

    // Apply sorting to ALL listings and re-render
    currentPage = 0;  // Reset to first page when sorting
    applyFiltersAndSort();
    renderListings();
}

function changePageSize() {
    const select = document.getElementById('pageSizeSelect');
    const newSize = parseInt(select.value, 10);

    if (newSize !== pageSize) {
        pageSize = newSize;
        localStorage.setItem('pageSize', newSize.toString());
        currentPage = 0; // Reset to first page
        renderListings(); // Client-side - just re-render with new page size
    }
}

function showTab(tabId) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    document.querySelector(`.tab[data-tab="${tabId}"]`).classList.add('active');
    document.getElementById(tabId).classList.add('active');

    // Load data when switching tabs
    if (tabId === 'listings') {
        loadListings();
    } else if (tabId === 'sync') {
        loadSyncTab();
    }
}

// Auth
async function checkAuthStatus() {
    try {
        const res = await secureFetch('/api/auth/status');
        const data = await res.json();
        isAuthenticated = data.authenticated;
        isConfigured = data.configured;
        updateAuthUI();
        await loadCurrentAccount(); // Load account info after updating auth UI
    } catch (err) {
        console.error('Auth check failed:', err);
    }
}

function updateAuthUI() {
    const dot = document.getElementById('statusDot');
    const text = document.getElementById('statusText');
    const btn = document.getElementById('authBtn');

    if (!isConfigured) {
        dot.classList.remove('connected');
        dot.style.background = 'var(--warning)';
        text.textContent = 'Not Configured';
        btn.textContent = 'Setup Required';
        btn.title = 'Set EBAY_CLIENT_ID and EBAY_CLIENT_SECRET environment variables';
    } else if (isAuthenticated) {
        dot.style.background = '';
        dot.classList.add('connected');
        text.textContent = 'Connected';
        btn.textContent = 'Reconnect';
        btn.title = '';
    } else {
        dot.style.background = '';
        dot.classList.remove('connected');
        text.textContent = 'Not Connected';
        btn.textContent = 'Connect to eBay';
        btn.title = '';
    }

    // Update account display when auth status changes
    updateAccountDisplay();
}

// Note: loadCurrentAccount() is defined later in the file (around line 917)
// It uses currentAccount object and has retry logic for auth timing

async function handleAuth() {
    try {
        const res = await secureFetch('/api/auth/url');
        const data = await res.json();

        // Check if the URL looks valid (has a client_id)
        if (!data.url || data.url.includes('client_id=&') || data.url.includes('client_id=""')) {
            alert('eBay API credentials not configured.\n\nSet these environment variables before starting the server:\n\nEBAY_CLIENT_ID=your-client-id\nEBAY_CLIENT_SECRET=your-client-secret');
            return;
        }

        // Open OAuth in new window
        const authWindow = window.open(data.url, 'ebay-auth', 'width=600,height=700');

        // Poll for auth completion
        const pollInterval = setInterval(async () => {
            try {
                const statusRes = await secureFetch('/api/auth/status');
                const statusData = await statusRes.json();

                if (statusData.authenticated) {
                    clearInterval(pollInterval);
                    if (authWindow && !authWindow.closed) {
                        authWindow.close();
                    }
                    await checkAuthStatus();
                    await loadCurrentAccount();  // Reload account details immediately
                    loadListings();  // Refresh listings after successful login
                }
            } catch (err) {
                console.error('Auth polling error:', err);
            }
        }, 1000);

        // Stop polling if window closed manually
        const closeCheck = setInterval(() => {
            if (authWindow && authWindow.closed) {
                clearInterval(pollInterval);
                clearInterval(closeCheck);
                checkAuthStatus(); // Update UI in case they completed it
            }
        }, 500);
    } catch (err) {
        alert('Failed to get auth URL: ' + err.message);
    }
}

// Reference data
async function loadReferenceData() {
    try {
        const [brandsRes, tariffsRes, bandsRes] = await Promise.all([
            secureFetch('/api/reference/brands'),
            secureFetch('/api/reference/tariffs'),
            secureFetch('/api/weight-bands')
        ]);

        const brandsData = await brandsRes.json();
        const tariffsData = await tariffsRes.json();
        const bandsData = await bandsRes.json();

        window.dbBrands = brandsData.brands || [];
        window.dbTariffs = tariffsData.tariffs || [];
        weightBands = bandsData.weightBands || [];

        // Legacy data for calculator compatibility
        brands = window.dbBrands.map(b => b.brandName);
        tariffCountries = window.dbTariffs.map(t => ({
            country: t.countryName,
            ratePercent: (t.tariffRate * 100).toFixed(0)
        }));

        populateBrandSelect();
        populateCOOSelect();
        populateReferenceTables();
    } catch (err) {
        console.error('Failed to load reference data:', err);
    }
}

function populateBrandSelect() {
    const select = document.getElementById('calcBrand');
    select.innerHTML = brands.map(b => `<option value="${b}">${b}</option>`).join('');
}

function populateCOOSelect() {
    const select = document.getElementById('calcCOO');
    const options = ['<option value="">Use Brand Default</option>'];
    tariffCountries.forEach(c => {
        options.push(`<option value="${c.country}">${c.country} (${c.ratePercent}%)</option>`);
    });
    select.innerHTML = options.join('');
}

function populateReferenceTables() {
    // Tariff table with edit/delete buttons
    const tariffBody = document.querySelector('#tariffTable tbody');
    tariffBody.innerHTML = window.dbTariffs.map(t =>
        `<tr>
            <td>${t.countryName}</td>
            <td>${(t.tariffRate * 100).toFixed(1)}%</td>
            <td>${t.notes || ''}</td>
            <td>
                <button class="btn btn-sm" onclick="editTariff(${t.id})">Edit</button>
                <button class="btn btn-sm btn-danger" onclick="deleteTariff(${t.id})">Delete</button>
            </td>
        </tr>`
    ).join('');

    // Brand table with edit/delete buttons
    const brandBody = document.querySelector('#brandTable tbody');
    brandBody.innerHTML = window.dbBrands.map(b => {
        const tariff = window.dbTariffs.find(t => t.countryName === b.primaryCoo);
        return `<tr>
            <td>${b.brandName}</td>
            <td>${b.primaryCoo}</td>
            <td>${tariff ? (tariff.tariffRate * 100).toFixed(1) + '%' : '-'}</td>
            <td>${b.notes || ''}</td>
            <td>
                <button class="btn btn-sm" onclick="editBrand(${b.id})">Edit</button>
                <button class="btn btn-sm btn-danger" onclick="deleteBrand(${b.id})">Delete</button>
            </td>
        </tr>`;
    }).join('');

    // Weight bands table (read-only)
    const weightBody = document.querySelector('#weightTable tbody');
    weightBody.innerHTML = weightBands.map(b =>
        `<tr><td>${b.key}</td><td>${b.label.replace(b.key + ' ', '')}</td><td>$${b.basePrice.toFixed(2)}</td></tr>`
    ).join('');
}

// Brand to COO mapping (client-side copy for display)
const brandCOOMap = {
    "Ada + Lou": "Indonesia", "Aje": "China", "Arnhem": "Indonesia",
    "Auguste": "China", "Blue Illusion": "China", "Camilla Franks": "India",
    "Coven & Co": "China", "Fillyboo": "Indonesia", "Free People": "China",
    "Ghanda": "Australia", "Innika Choo [Bali]": "Indonesia",
    "Innika Choo [China]": "China", "Innika Choo [India]": "India",
    "Jen's Pirate Booty": "Mexico", "Kivari": "China", "Kip & Co": "India",
    "Lack of Color": "China", "Lele Sadoughi": "United States",
    "Love Bonfire": "China", "LoveShackFancy": "China",
    "Nine Lives Bazaar": "China", "Reebok x Maison": "Vietnam",
    "Sabbi": "Australia", "Selkie": "China", "Spell": "China",
    "Tree of Life": "India", "Wildfox": "China"
};

function getBrandCOO(brand) {
    return brandCOOMap[brand] || 'China';
}

// Convert eBay thumbnail URL to full-size image URL
function getFullSizeImageUrl(thumbnailUrl) {
    if (!thumbnailUrl || thumbnailUrl === 'https://via.placeholder.com/50') {
        return thumbnailUrl;
    }
    // eBay image URLs typically have size parameters like s-l64, s-l140, s-l225, s-l500
    // Replace with s-l1600 for full size (1600px max dimension)
    return thumbnailUrl.replace(/\/s-l\d+\./, '/s-l1600.');
}

// Calculator
async function calculate() {
    const params = {
        itemValueAUD: parseFloat(document.getElementById('calcValue').value) || 0,
        weightBand: document.getElementById('calcWeight').value,
        brandName: document.getElementById('calcBrand').value,
        countryOfOrigin: document.getElementById('calcCOO').value,
        includeExtraCover: document.getElementById('calcExtraCover').checked,
        discountBand: parseInt(document.getElementById('calcDiscount').value) || 0
    };

    try {
        const res = await secureFetch('/api/calculate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(params)
        });
        const result = await res.json();

        if (result.error) {
            alert('Error: ' + result.error);
            return;
        }

        displayCalculationResult(result);
    } catch (err) {
        alert('Calculation failed: ' + err.message);
    }
}

function displayCalculationResult(result) {
    const resultBox = document.getElementById('calcResult');
    const totalEl = document.getElementById('resultTotal');
    const breakdownEl = document.getElementById('resultBreakdown');
    const warningBox = document.getElementById('calcWarning');

    totalEl.textContent = result.totalShipping.toFixed(2);

    const b = result.breakdown;
    breakdownEl.innerHTML = `
        <strong>Breakdown:</strong><br>
        AusPost Shipping: $${b.ausPostShipping.toFixed(2)}<br>
        ${b.extraCover > 0 ? `Extra Cover: $${b.extraCover.toFixed(2)}<br>` : ''}
        Shipping Subtotal: $${b.shippingSubtotal.toFixed(2)}<br>
        <br>
        Tariff Duties (${(result.inputs.tariffRate * 100).toFixed(0)}%): $${b.tariffDuties.toFixed(2)}<br>
        Zonos Fees: $${b.zonosFees.toFixed(2)}<br>
        Duties Subtotal: $${b.dutiesSubtotal.toFixed(2)}<br>
        <br>
        <strong>COO:</strong> ${result.inputs.countryOfOrigin}
    `;

    resultBox.style.display = 'block';

    if (result.warnings.extraCoverRecommended) {
        warningBox.innerHTML = '⚠️ Extra Cover recommended for items $250+ AUD';
        warningBox.style.display = 'block';
    } else {
        warningBox.style.display = 'none';
    }
}

// Listings - now fetches ALL listings progressively
async function loadListings(forceRefresh = false) {
    if (!isAuthenticated) {
        document.getElementById('listingsLoading').style.display = 'none';
        document.getElementById('listingsEmpty').style.display = 'block';
        document.getElementById('listingsEmpty').textContent = 'Please connect to eBay first.';
        return;
    }

    // If we already have all listings cached and not forcing refresh, just re-render
    if (allListings.length > 0 && !forceRefresh) {
        applyFiltersAndSort();
        renderListings();
        return;
    }

    // Start fresh
    if (forceRefresh) {
        allListings = [];
        enrichedDataCache.clear();
        stopEnrichmentPolling();
    }

    document.getElementById('listingsLoading').style.display = 'flex';
    document.getElementById('listingsTable').style.display = 'none';
    document.getElementById('listingsEmpty').style.display = 'none';
    isLoadingAllListings = true;
    listingsLoadProgress = 0;

    try {
        // First fetch to get total count (pass force=true if refreshing)
        const forceParam = forceRefresh ? '&force=true' : '';
        const firstRes = await secureFetch(`/api/offers?limit=50&offset=0${forceParam}`);
        const firstData = await firstRes.json();

        if (firstData.error) throw new Error(firstData.error);

        totalListingsFromAPI = firstData.total || 0;
        allListings = firstData.offers || [];
        listingsLoadProgress = allListings.length;

        updateLoadingProgress();

        // Show first page immediately while loading the rest
        applyFiltersAndSort();
        renderListings();
        document.getElementById('listingsTable').style.display = 'block';

        // Fetch remaining pages in background
        const batchSize = 50;
        let offset = batchSize;


        while (offset < totalListingsFromAPI) {
            const res = await secureFetch(`/api/offers?limit=${batchSize}&offset=${offset}${forceParam}`);
            const data = await res.json();

            if (data.error) {
                console.error(`[LOAD-ALL] API error at offset ${offset}:`, data.error);
                throw new Error(data.error);
            }

            const newOffers = data.offers || [];
            allListings = [...allListings, ...newOffers];
            listingsLoadProgress = allListings.length;
            offset += batchSize;

            updateLoadingProgress();

            // Re-render to show updated count
            applyFiltersAndSort();
            renderListings();
        }

        isLoadingAllListings = false;

        // Hide loading indicator and re-render
        document.getElementById('listingsLoading').style.display = 'none';
        renderListings();

        // Now enrich ALL items in batches
        enrichAllItemsInBatches();

    } catch (err) {
        console.error('Failed to load listings:', err);
        isLoadingAllListings = false;
        document.getElementById('listingsLoading').style.display = 'none';
        document.getElementById('listingsEmpty').style.display = 'block';
        document.getElementById('listingsEmpty').textContent = 'Failed to load: ' + err.message;
    }
}

// Update loading progress indicator
function updateLoadingProgress() {
    const loadingEl = document.getElementById('listingsLoading');
    if (loadingEl && isLoadingAllListings) {
        loadingEl.innerHTML = `<div class="spinner"></div><span>Loading ${listingsLoadProgress}/${totalListingsFromAPI} listings...</span>`;
    }
}

// Apply search filter and sorting to allListings
function applyFiltersAndSort() {
    // Start with all listings
    let filtered = [...allListings];

    // Apply search filter - searches title, brand, and COO
    if (searchFilter) {
        const term = searchFilter.toLowerCase();
        filtered = filtered.filter(offer => {
            const title = (offer.title || '').toLowerCase();
            const enriched = enrichedDataCache.get(offer.offerId);
            const brand = (enriched?.brand || offer.brand || '').toLowerCase();
            const coo = (enriched?.countryOfOrigin || '').toLowerCase();
            return title.includes(term) || brand.includes(term) || coo.includes(term);
        });
    }

    // Apply sorting - default to newest first (by offerId descending) when no column selected
    // eBay offer/item IDs are numeric and higher = newer listings
    filtered.sort((a, b) => {
        let aVal, bVal;
        let direction = sortDirection;

        // If no column selected, default to offerId descending (newest first)
        const column = sortColumn || 'offerId';
        if (!sortColumn) {
            direction = 'desc';  // Default to descending for newest first
        }

        switch (column) {
            case 'offerId':
                // Use BigInt for large item IDs to avoid precision issues
                aVal = BigInt(a.offerId || '0');
                bVal = BigInt(b.offerId || '0');
                // Return comparison for BigInt (can't use simple subtraction)
                if (aVal === bVal) return 0;
                const result = aVal > bVal ? 1 : -1;
                return direction === 'asc' ? result : -result;
            case 'title':
                aVal = (a.title || '').toLowerCase();
                bVal = (b.title || '').toLowerCase();
                break;
            case 'price':
                aVal = parseFloat(a.pricingSummary?.price?.value || '0');
                bVal = parseFloat(b.pricingSummary?.price?.value || '0');
                break;
            case 'brand':
                aVal = (enrichedDataCache.get(a.offerId)?.brand || a.brand || '').toLowerCase();
                bVal = (enrichedDataCache.get(b.offerId)?.brand || b.brand || '').toLowerCase();
                break;
            case 'coo':
                aVal = (enrichedDataCache.get(a.offerId)?.countryOfOrigin || '').toLowerCase();
                bVal = (enrichedDataCache.get(b.offerId)?.countryOfOrigin || '').toLowerCase();
                break;
            case 'shipping':
                aVal = parseFloat(enrichedDataCache.get(a.offerId)?.shippingCost || '0');
                bVal = parseFloat(enrichedDataCache.get(b.offerId)?.shippingCost || '0');
                break;
            case 'calculated':
                aVal = calculationCache.get(a.offerId)?.calculatedCost || 0;
                bVal = calculationCache.get(b.offerId)?.calculatedCost || 0;
                break;
            case 'diff':
                aVal = calculationCache.get(a.offerId)?.diff || 0;
                bVal = calculationCache.get(b.offerId)?.diff || 0;
                break;
            default:
                aVal = '';
                bVal = '';
        }

        if (typeof aVal === 'string') {
            return direction === 'asc' ? aVal.localeCompare(bVal) : bVal.localeCompare(aVal);
        }
        return direction === 'asc' ? aVal - bVal : bVal - aVal;
    });

    // Store filtered results for pagination
    listings = filtered;
    totalListings = filtered.length;
}

// Update enrichment progress display with graphical bar
function updateEnrichmentProgress() {
    const totalItems = allListings.length;
    const enrichedItems = enrichedDataCache.size;
    const percent = totalItems > 0 ? Math.round((enrichedItems / totalItems) * 100) : 0;


    // Update both top and bottom progress indicators
    const elements = [
        { container: 'enrichmentProgressTop', bar: 'enrichmentBarTop', text: 'enrichmentTextTop' },
        { container: 'enrichmentProgress', bar: 'enrichmentBar', text: 'enrichmentText' }
    ];

    elements.forEach(({ container, bar, text }) => {
        const containerEl = document.getElementById(container);
        const barEl = document.getElementById(bar);
        const textEl = document.getElementById(text);

        if (containerEl && barEl && textEl) {
            if (percent < 100 && totalItems > 0) {
                containerEl.style.display = 'inline-flex';
                barEl.style.width = `${percent}%`;
                textEl.textContent = `${percent}%`;
            } else if (percent >= 100) {
                // Hide after completion (with brief delay to show 100%)
                barEl.style.width = '100%';
                textEl.textContent = '100%';
                setTimeout(() => {
                    containerEl.style.display = 'none';
                }, 1500);
            }
        }
    });
}

// Enrich ALL items in batches of 60, with 2 batches in parallel
// Backend processes up to 30 items concurrently per request
async function enrichAllItemsInBatches() {
    const batchSize = 60;
    const parallelBatches = 2; // Send 2 batches simultaneously
    const itemIds = allListings.map(l => l.offerId);

    console.log(`[ENRICH-ALL] Starting enrichment of ${itemIds.length} items (batch=${batchSize}, parallel=${parallelBatches})`);
    updateEnrichmentProgress();

    // Process a single batch and return results
    async function processBatch(batch, batchNum, totalBatches) {
        const batchIds = batch.join(',');
        console.log(`[ENRICH-ALL] Batch ${batchNum}/${totalBatches}: ${batch.length} items`);

        const res = await secureFetch(`/api/offers/enriched?itemIds=${encodeURIComponent(batchIds)}`);
        const data = await res.json();

        // Update cache for each item in batch
        for (const [itemId, enrichedData] of Object.entries(data)) {
            const enrichedWithId = { ...enrichedData, itemId };
            enrichedDataCache.set(itemId, enrichedWithId);
            updateTableRow(enrichedWithId);
        }

        console.log(`[ENRICH-ALL] Batch ${batchNum} complete: ${Object.keys(data).length} items`);
        updateEnrichmentProgress();
        return Object.keys(data).length;
    }

    // Split into batches
    const batches = [];
    for (let i = 0; i < itemIds.length; i += batchSize) {
        batches.push(itemIds.slice(i, i + batchSize));
    }
    const totalBatches = batches.length;

    // Process batches in parallel groups
    for (let i = 0; i < batches.length; i += parallelBatches) {
        const batchGroup = batches.slice(i, i + parallelBatches);
        const promises = batchGroup.map((batch, idx) =>
            processBatch(batch, i + idx + 1, totalBatches).catch(err => {
                console.error(`[ENRICH-ALL] Batch ${i + idx + 1} failed:`, err);
                return 0;
            })
        );
        await Promise.all(promises);
    }

    console.log(`[ENRICH-ALL] Complete: ${enrichedDataCache.size} items enriched`);

    // Now fetch calculations from backend (keeps business logic server-side)
    await fetchCalculationsFromBackend();
}

// Fetch postage calculations from backend API
// This keeps all business logic (COO matching, tariff rates, postage formula) on the server
async function fetchCalculationsFromBackend() {
    if (allListings.length === 0) return;

    console.log(`[CALC] Fetching calculations for ${allListings.length} items from backend`);

    // Build request with item IDs and prices
    const items = allListings.map(offer => ({
        itemId: offer.offerId,
        price: parseFloat(offer.pricingSummary?.price?.value || '0')
    }));

    try {
        const res = await secureFetch('/api/calculate/batch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(items)
        });
        const data = await res.json();

        // Store calculation results in cache
        for (const [itemId, calcData] of Object.entries(data)) {
            calculationCache.set(itemId, calcData);
        }

        console.log(`[CALC] Received calculations for ${Object.keys(data).length} items`);

        // Re-render to show calculated values
        applyFiltersAndSort();
        renderListings();

    } catch (err) {
        console.error('[CALC] Failed to fetch calculations:', err);
    }
}

async function renderListings() {
    // Only hide loading indicator when ALL listings have loaded
    if (!isLoadingAllListings) {
        document.getElementById('listingsLoading').style.display = 'none';
    } else {
        // Keep showing loading progress during progressive load
        updateLoadingProgress();
    }

    if (listings.length === 0 && !isLoadingAllListings) {
        document.getElementById('listingsEmpty').style.display = 'block';
        document.getElementById('listingsEmpty').textContent = 'No active listings found.';
        return;
    }

    document.getElementById('listingsTable').style.display = 'block';
    document.getElementById('listingsPagination').style.display = 'flex';

    // Client-side pagination: get slice of filtered listings for current page
    const startIndex = currentPage * pageSize;
    const endIndex = startIndex + pageSize;
    const pageListings = listings.slice(startIndex, endIndex);

    const tbody = document.getElementById('listingsBody');
    const rows = pageListings.map((offer) => {
        const price = offer.pricingSummary?.price?.value || '0';
        const title = offer.title || '-';
        const imageUrl = offer.image?.imageUrl || 'https://via.placeholder.com/50';
        const listingUrl = `https://www.ebay.com.au/itm/${offer.offerId}`;

        // Store full-size image URLs for carousel
        // Convert thumbnail to full-size (1600px) for better quality viewing
        // Only set if we don't already have enriched images (prevents overwriting multi-image arrays)
        const fullSizeImageUrl = getFullSizeImageUrl(imageUrl);
        if (!carouselImages.has(offer.offerId) || carouselImages.get(offer.offerId).length <= 1) {
            carouselImages.set(offer.offerId, [fullSizeImageUrl]);
        }

        // Get enriched data from cache (if available)
        const enriched = enrichedDataCache.get(offer.offerId);

        // Brand: show spinner if enrichment hasn't loaded yet, then validate against title
        let brand = offer.brand || '-';
        let brandDisplay = brand;
        let brandClass = '';

        if (!enriched && brand === '-') {
            // Still loading - show spinner
            brandDisplay = '<div class="spinner-inline"></div>';
        } else if (enriched?.brand) {
            brand = enriched.brand;
            // Validate: Brand must be present AND appear in title
            if (!brand || brand === '-' || brand.trim() === '') {
                // Brand is missing
                brandClass = 'brand-missing';
                brandDisplay = '<strong>[MISSING]</strong>';
            } else if (!title.toLowerCase().includes(brand.toLowerCase())) {
                // Brand is set but NOT in title - mismatch
                brandClass = 'brand-mismatch';
                brandDisplay = `${brand}<br><strong>[NOT IN TITLE]</strong>`;
            } else {
                // Brand is set AND in title - all good
                brandClass = 'brand-match';
                brandDisplay = brand;
            }
        } else if (enriched && (!enriched.brand || enriched.brand === '-' || enriched.brand.trim() === '')) {
            // Enrichment loaded but no brand found
            brandClass = 'brand-missing';
            brandDisplay = '<strong>[MISSING]</strong>';
        }

        // COO: show spinner if enrichment hasn't loaded yet, otherwise show COO with validation
        const coo = enriched?.countryOfOrigin || '-';
        const expectedCOO = getBrandCOO(brand);  // Always returns a value (fallback to 'China')
        let cooClass = '';
        let cooDisplay = coo;

        if (!enriched) {
            // Still loading enriched data - show spinner
            cooDisplay = '<div class="spinner-inline"></div>';
        } else if (coo !== '-') {
            // COO exists in listing
            if (coo.toLowerCase() === expectedCOO.toLowerCase()) {
                cooClass = 'coo-match';
                cooDisplay = coo;
            } else {
                cooClass = 'coo-mismatch';
                cooDisplay = `${coo}<br><strong>[MISMATCH: ${expectedCOO}]</strong>`;
            }
        } else {
            // COO missing from listing - show expected COO with [MISSING] label
            cooClass = 'coo-missing';
            cooDisplay = `${expectedCOO}<br><strong>[MISSING]</strong>`;
        }

        // US Postage: show spinner if enrichment hasn't loaded yet
        let currentUSPostage = '-';
        if (!enriched) {
            currentUSPostage = '<div class="spinner-inline"></div>';
        } else if (enriched?.shippingCost) {
            currentUSPostage = '$' + parseFloat(enriched.shippingCost).toFixed(2);
        } else if (offer.shippingCost?.value) {
            currentUSPostage = '$' + parseFloat(offer.shippingCost.value).toFixed(2);
        }

        // Get calculated postage from BACKEND (no client-side calculation!)
        // Backend does all business logic: COO matching, tariff lookup, postage calculation
        let calculated = '-';
        let diff = '-';
        let diffClass = '';

        const calcData = calculationCache.get(offer.offerId);

        if (!enriched) {
            // Still loading enrichment data - show spinner
            calculated = '<div class="spinner-inline"></div>';
            diff = '<div class="spinner-inline"></div>';
        } else if (!calcData) {
            // Enrichment loaded but calculation not yet fetched - show spinner
            calculated = '<div class="spinner-inline"></div>';
            diff = '<div class="spinner-inline"></div>';
        } else if (calcData.cooStatus === 'missing') {
            // COO is missing - backend tells us this
            calculated = '<strong class="coo-missing">No COO set!</strong>';
            diff = '<strong class="coo-missing">No COO set!</strong>';
            diffClass = 'coo-missing';
        } else {
            // Display backend-calculated values
            calculated = '$' + calcData.calculatedCost.toFixed(2);
            diffClass = calcData.diffStatus === 'ok' ? 'diff-ok' : 'diff-bad';
            const sign = calcData.diff >= 0 ? '+' : '';
            diff = sign + '$' + calcData.diff.toFixed(2);
        }

        return `
            <tr data-offer-id="${offer.offerId}">
                <td class="checkbox-cell">
                    <input type="checkbox" onchange="toggleSelect('${offer.offerId}')"
                           ${selectedItems.has(offer.offerId) ? 'checked' : ''}>
                </td>
                <td><img src="${imageUrl}" class="thumbnail" alt="${title}" onclick="openCarousel('${offer.offerId}')" onerror="this.src='https://via.placeholder.com/50'"></td>
                <td class="title-cell"><a href="${listingUrl}" target="_blank" rel="noopener noreferrer" class="title-link">${title}</a></td>
                <td class="price">$${parseFloat(price).toFixed(2)}</td>
                <td class="brand-cell ${brandClass}" data-item-id="${offer.offerId}">${brandDisplay}</td>
                <td class="coo-cell ${cooClass}" data-item-id="${offer.offerId}">${cooDisplay}</td>
                <td>Medium</td>
                <td class="shipping-cell" data-item-id="${offer.offerId}">${currentUSPostage}</td>
                <td class="calculated-cell" data-item-id="${offer.offerId}">${calculated}</td>
                <td class="diff-cell ${diffClass}" data-item-id="${offer.offerId}">${diff}</td>
                <td>
                    <button class="btn btn-sm btn-secondary" onclick="editItem('${offer.offerId}')">Edit</button>
                </td>
            </tr>
        `;
    });

    tbody.innerHTML = rows.join('');

    // Update pagination (both top and bottom) - now using client-side pagination
    const start = listings.length > 0 ? currentPage * pageSize + 1 : 0;
    const end = Math.min(start + pageListings.length - 1, totalListings);
    document.getElementById('pageStart').textContent = start;
    document.getElementById('pageEnd').textContent = end;
    document.getElementById('pageTotal').textContent = totalListings;
    document.getElementById('pageStartTop').textContent = start;
    document.getElementById('pageEndTop').textContent = end;
    document.getElementById('pageTotalTop').textContent = totalListings;

    // Show loading indicator if still fetching
    if (isLoadingAllListings) {
        document.getElementById('pageTotal').textContent = `${listingsLoadProgress}/${totalListingsFromAPI} (loading...)`;
        document.getElementById('pageTotalTop').textContent = `${listingsLoadProgress}/${totalListingsFromAPI} (loading...)`;
    }

    const isPrevDisabled = currentPage === 0;
    const isNextDisabled = end >= totalListings;
    document.getElementById('prevPage').disabled = isPrevDisabled;
    document.getElementById('nextPage').disabled = isNextDisabled;
    document.getElementById('prevPageTop').disabled = isPrevDisabled;
    document.getElementById('nextPageTop').disabled = isNextDisabled;

    // Show/hide top pagination
    document.getElementById('listingsPaginationTop').style.display = 'flex';

    // Note: Enrichment now happens via enrichAllItemsInBatches() after all listings load
}

function prevPage() {
    if (currentPage > 0) {
        currentPage--;
        renderListings();  // Client-side pagination - just re-render
    }
}

function nextPage() {
    const maxPage = Math.ceil(totalListings / pageSize) - 1;
    if (currentPage < maxPage) {
        currentPage++;
        renderListings();  // Client-side pagination - just re-render
    }
}

function toggleSelect(offerId) {
    if (selectedItems.has(offerId)) {
        selectedItems.delete(offerId);
    } else {
        selectedItems.add(offerId);
    }
    updateActionBar();
}

function toggleSelectAll() {
    const checked = document.getElementById('selectAll').checked;
    if (checked) {
        listings.forEach(o => selectedItems.add(o.offerId));
    } else {
        selectedItems.clear();
    }
    renderListings();
    updateActionBar();
}

function updateActionBar() {
    const bar = document.getElementById('actionBar');
    const count = document.getElementById('selectedCount');

    if (selectedItems.size > 0) {
        bar.classList.remove('hidden');
        count.textContent = selectedItems.size;
    } else {
        bar.classList.add('hidden');
    }
}

function editItem(offerId) {
    const offer = listings.find(o => o.offerId === offerId);
    if (offer) {
        // For now, just show an alert - in Phase 2, open a modal
        alert(`Edit offer: ${offerId}\nSKU: ${offer.sku}`);
    }
}

async function bulkResolve() {
    if (selectedItems.size === 0) return;

    if (!confirm(`Update shipping for ${selectedItems.size} items?`)) return;

    alert('Bulk update will be implemented in Phase 2');
    // TODO: Loop through selectedItems and call /api/update-shipping
}

// Brand → Country of Origin (COO) mapping
// Note: Postage calculation is now done server-side via /api/calculate/batch
// The backend handles all business logic: COO matching, tariff rates, postage formula
// See internal/calculator/calculator.go for the calculation implementation

// Account management
async function loadCurrentAccount(retryCount = 0) {
    try {
        const res = await secureFetch('/api/account/current');
        const data = await res.json();

        if (data.configured && data.account) {
            currentAccount = data.account;
            updateAccountDisplay();
        } else {
            // If we just authenticated but account isn't ready yet, retry up to 3 times
            if (isAuthenticated && retryCount < 3) {
                setTimeout(() => loadCurrentAccount(retryCount + 1), 500);
            }
        }
    } catch (err) {
        console.error('Failed to load current account:', err);
    }
}

function updateAccountDisplay() {
    const accountInfo = document.getElementById('accountInfo');
    const accountName = document.getElementById('accountName');
    const accountEnv = document.getElementById('accountEnv');

    if (currentAccount) {
        // Update account info badges on the right
        accountName.textContent = currentAccount.displayName;
        const envBadge = currentAccount.environment === 'production' ? '🔴 PROD' : '🟡 SANDBOX';
        accountEnv.textContent = `[${envBadge}]`;
        accountInfo.style.display = 'block';
    } else {
        accountInfo.style.display = 'none';
    }
}

async function loadAvailableAccounts() {
    try {
        const res = await secureFetch('/api/accounts');
        const data = await res.json();
        availableAccounts = data.accounts || [];

        // Populate source account select
        const select = document.getElementById('sourceAccountSelect');
        const options = ['<option value="">Select account...</option>'];
        availableAccounts.forEach(acc => {
            // Don't show current account as import source
            if (!currentAccount || acc.accountKey !== currentAccount.accountKey) {
                options.push(`<option value="${acc.accountKey}">${acc.displayName}</option>`);
            }
        });
        select.innerHTML = options.join('');
    } catch (err) {
        console.error('Failed to load available accounts:', err);
    }
}

// Sync tab
async function loadSyncTab() {
    await checkAuthStatus(); // Update auth status first
    await loadAvailableAccounts();
    await loadSyncHistory();
    updateSyncAccountDisplay();
}

function updateSyncAccountDisplay() {
    const display = document.getElementById('currentAccountDisplay');

    if (!currentAccount) {
        display.innerHTML = '<p style="color: var(--warning);">⚠️ Not connected to an eBay account. Click "Connect to eBay" above to authenticate.</p>';
        document.getElementById('exportBtn').disabled = true;
        document.getElementById('importBtn').disabled = true;
    } else {
        const envBadge = currentAccount.environment === 'production' ?
            '<span style="color: var(--danger);">🔴 PRODUCTION</span>' :
            '<span style="color: var(--warning);">🟡 SANDBOX</span>';
        const lastExport = currentAccount.lastExportAt ?
            new Date(currentAccount.lastExportAt).toLocaleString() :
            'Never';

        display.innerHTML = `
            <div style="font-size: 0.875rem;">
                <div><strong>Name:</strong> ${currentAccount.displayName}</div>
                <div><strong>Environment:</strong> ${envBadge}</div>
                <div><strong>Marketplace:</strong> ${currentAccount.marketplaceId}</div>
                <div><strong>Last Export:</strong> ${lastExport}</div>
                ${!isAuthenticated ? '<div style="margin-top: 0.5rem; color: var(--warning);">⚠️ Not authenticated - connect to eBay to use sync features</div>' : ''}
            </div>
        `;

        document.getElementById('exportBtn').disabled = !isAuthenticated;
        document.getElementById('importBtn').disabled = !isAuthenticated;
    }
}

async function exportData() {
    if (!currentAccount) {
        alert('No account configured. Restart server with -store flag.');
        return;
    }

    if (!isAuthenticated) {
        alert('Please connect to eBay first.');
        return;
    }

    if (!confirm(`Export all data from ${currentAccount.displayName} to database?\n\nThis will save:\n- Business policies\n- Inventory items\n- Offers/Listings`)) {
        return;
    }

    const statusDiv = document.getElementById('syncStatus');
    const statusText = document.getElementById('syncStatusText');
    const exportBtn = document.getElementById('exportBtn');

    exportBtn.disabled = true;
    statusDiv.style.display = 'block';
    statusText.textContent = 'Exporting data...';

    try {
        const res = await secureFetch('/api/sync/export', { method: 'POST' });
        const data = await res.json();

        if (data.error) {
            throw new Error(data.error);
        }

        statusDiv.style.display = 'none';
        alert(`Export successful!\n\n${data.message}`);

        // Reload account info and history
        await loadCurrentAccount();
        await loadSyncHistory();
        updateSyncAccountDisplay();
    } catch (err) {
        statusDiv.style.display = 'none';
        alert('Export failed: ' + err.message);
    } finally {
        exportBtn.disabled = false;
    }
}

async function importData() {
    const sourceAccountKey = document.getElementById('sourceAccountSelect').value;

    if (!sourceAccountKey) {
        alert('Please select a source account to import from.');
        return;
    }

    if (!currentAccount) {
        alert('No account configured. Restart server with -store flag.');
        return;
    }

    if (!isAuthenticated) {
        alert('Please connect to eBay first.');
        return;
    }

    const sourceAccount = availableAccounts.find(a => a.accountKey === sourceAccountKey);
    if (!sourceAccount) {
        alert('Source account not found.');
        return;
    }

    if (!confirm(`Import data from ${sourceAccount.displayName} to ${currentAccount.displayName}?\n\nThis will create inventory items in your current eBay account.`)) {
        return;
    }

    const statusDiv = document.getElementById('syncStatus');
    const statusText = document.getElementById('syncStatusText');
    const importBtn = document.getElementById('importBtn');

    importBtn.disabled = true;
    statusDiv.style.display = 'block';
    statusText.textContent = 'Importing data...';

    try {
        const res = await secureFetch('/api/sync/import', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ sourceAccountKey })
        });
        const data = await res.json();

        if (data.error) {
            throw new Error(data.error);
        }

        statusDiv.style.display = 'none';
        alert(`Import successful!\n\n${data.message}`);

        // Reload history
        await loadSyncHistory();
    } catch (err) {
        statusDiv.style.display = 'none';
        alert('Import failed: ' + err.message);
    } finally {
        importBtn.disabled = false;
    }
}

async function loadSyncHistory() {
    const loadingDiv = document.getElementById('historyLoading');
    const tableDiv = document.getElementById('historyTable');
    const emptyDiv = document.getElementById('historyEmpty');
    const tbody = document.getElementById('historyBody');

    loadingDiv.style.display = 'flex';
    tableDiv.style.display = 'none';
    emptyDiv.style.display = 'none';

    try {
        const res = await secureFetch('/api/sync/history');
        const data = await res.json();

        loadingDiv.style.display = 'none';

        if (!data.history || data.history.length === 0) {
            emptyDiv.style.display = 'block';
            return;
        }

        tableDiv.style.display = 'block';

        tbody.innerHTML = data.history.map(h => {
            const started = new Date(h.startedAt).toLocaleString();
            let duration = '-';
            if (h.completedAt) {
                const start = new Date(h.startedAt);
                const end = new Date(h.completedAt);
                const seconds = Math.round((end - start) / 1000);
                duration = seconds + 's';
            }

            let statusClass = '';
            let statusText = h.status;
            if (h.status === 'success') {
                statusClass = 'style="color: var(--success);"';
                statusText = '✓ Success';
            } else if (h.status === 'failed') {
                statusClass = 'style="color: var(--danger);"';
                statusText = '✗ Failed';
            } else if (h.status === 'partial') {
                statusClass = 'style="color: var(--warning);"';
                statusText = '⚠ Partial';
            } else if (h.status === 'running') {
                statusText = '⏳ Running...';
            }

            const typeText = h.syncType === 'export' ? '📤 Export' : '📥 Import';
            const errorText = h.errorMessage || '-';

            return `
                <tr>
                    <td>${typeText}</td>
                    <td ${statusClass}>${statusText}</td>
                    <td>${h.itemsSynced}</td>
                    <td>${started}</td>
                    <td>${duration}</td>
                    <td style="font-size: 0.75rem; color: var(--text-muted);">${errorText.substring(0, 50)}</td>
                </tr>
            `;
        }).join('');
    } catch (err) {
        loadingDiv.style.display = 'none';
        emptyDiv.style.display = 'block';
        emptyDiv.textContent = 'Failed to load sync history: ' + err.message;
    }
}

// Progressive enrichment polling
function startEnrichmentPolling() {
    // Stop any existing polling
    stopEnrichmentPolling();

    if (listings.length === 0) return;

    // Poll every 2 seconds for enriched data
    enrichmentPollInterval = setInterval(async () => {
        await fetchEnrichedData();
    }, 2000);

    // Also fetch immediately
    fetchEnrichedData();
}

function stopEnrichmentPolling() {
    if (enrichmentPollInterval) {
        clearInterval(enrichmentPollInterval);
        enrichmentPollInterval = null;
    }
}

async function fetchEnrichedData() {
    if (listings.length === 0) return;

    // Get item IDs that need enrichment
    const itemIds = listings.map(l => l.offerId).join(',');

    try {
        console.log('[ENRICHMENT-DEBUG] Fetching enriched data for:', itemIds.split(',').length, 'items');
        const res = await secureFetch(`/api/offers/enriched?itemIds=${encodeURIComponent(itemIds)}`);
        const data = await res.json();

        console.log('[ENRICHMENT-DEBUG] Response data:', data);

        // Backend returns data as object with item IDs as keys: {itemId: {...enrichedData}}
        const enrichedEntries = Object.entries(data);
        console.log('[ENRICHMENT-DEBUG] enrichedEntries count:', enrichedEntries.length);

        if (enrichedEntries.length > 0) {
            let updated = 0;

            // Update cache and UI for each enriched item
            for (const [itemId, enrichedData] of enrichedEntries) {
                console.log('[ENRICHMENT-DEBUG] Processing item:', itemId, 'brand:', enrichedData.brand, 'images:', enrichedData.images?.length || 0);
                const existing = enrichedDataCache.get(itemId);

                // Only update if this is new or has changed
                if (!existing ||
                    existing.brand !== enrichedData.brand ||
                    existing.shippingCost !== enrichedData.shippingCost) {

                    console.log('[ENRICHMENT-DEBUG] Updating cache and table for item:', itemId);
                    // Add itemId to enrichedData object before caching
                    const enrichedWithId = { ...enrichedData, itemId };
                    enrichedDataCache.set(itemId, enrichedWithId);
                    updateTableRow(enrichedWithId);
                    updated++;
                }
            }

            console.log(`[ENRICHMENT] Updated ${updated}/${listings.length} items`);

            // Stop polling if all items have been enriched
            if (enrichedEntries.length >= listings.length) {
                console.log(`[ENRICHMENT] All ${listings.length} items enriched, stopping polling`);
                stopEnrichmentPolling();
            }
        } else {
            console.log('[ENRICHMENT-DEBUG] No enriched items in response');
        }
    } catch (err) {
        console.error('[ENRICHMENT] Failed to fetch enriched data:', err);
    }
}

function updateTableRow(enrichedData) {
    const { itemId, brand, shippingCost, shippingCurrency, countryOfOrigin, images } = enrichedData;

    // Update carousel images if we have enriched images
    console.log('[CAROUSEL-DEBUG] updateTableRow itemId:', itemId, 'images received:', images?.length || 0);
    if (images && images.length > 0) {
        carouselImages.set(itemId, images);
        console.log('[CAROUSEL-DEBUG] Stored', images.length, 'images for item:', itemId);
    }

    // Update Brand cell with validation
    const brandCell = document.querySelector(`.brand-cell[data-item-id="${itemId}"]`);
    if (brandCell) {
        // Get the listing's title for brand validation
        const offer = allListings.find(o => o.offerId === itemId);
        const title = offer?.title || '';

        let brandClass = '';
        let brandDisplay = '';

        if (!brand || brand === '-' || brand.trim() === '') {
            // Brand is missing
            brandClass = 'brand-missing';
            brandDisplay = '<strong>[MISSING]</strong>';
        } else if (!title.toLowerCase().includes(brand.toLowerCase())) {
            // Brand is set but NOT in title - mismatch
            brandClass = 'brand-mismatch';
            brandDisplay = `${brand}<br><strong>[NOT IN TITLE]</strong>`;
        } else {
            // Brand is set AND in title - all good
            brandClass = 'brand-match';
            brandDisplay = brand;
        }

        brandCell.innerHTML = brandDisplay;
        brandCell.className = `brand-cell ${brandClass}`;
        brandCell.setAttribute('data-item-id', itemId);
    }

    // Update COO cell
    const cooCell = document.querySelector(`.coo-cell[data-item-id="${itemId}"]`);
    if (cooCell) {
        const currentBrand = brand || document.querySelector(`.brand-cell[data-item-id="${itemId}"]`)?.textContent || '-';
        const coo = countryOfOrigin || '-';
        const expectedCOO = getBrandCOO(currentBrand);  // Always returns a value (fallback to 'China')

        let cooClass = '';
        let cooDisplay = coo;

        if (coo !== '-') {
            // COO exists in listing
            if (coo.toLowerCase() === expectedCOO.toLowerCase()) {
                cooClass = 'coo-match';
                cooDisplay = coo;
            } else {
                cooClass = 'coo-mismatch';
                cooDisplay = `${coo}<br><strong>[MISMATCH: ${expectedCOO}]</strong>`;
            }
        } else {
            // COO missing from listing - show expected COO with [MISSING] label
            cooClass = 'coo-missing';
            cooDisplay = `${expectedCOO}<br><strong>[MISSING]</strong>`;
        }

        cooCell.innerHTML = cooDisplay;
        cooCell.className = `coo-cell ${cooClass}`;
        cooCell.setAttribute('data-item-id', itemId);
    }

    // Update Shipping cell
    if (shippingCost) {
        const shippingCell = document.querySelector(`.shipping-cell[data-item-id="${itemId}"]`);
        if (shippingCell) {
            const formattedCost = '$' + parseFloat(shippingCost).toFixed(2);
            if (shippingCell.textContent !== formattedCost) {
                shippingCell.textContent = formattedCost;
            }
        }
    }

    // Update Calculated and Diff cells from BACKEND calculation cache
    // Note: Calculations are fetched separately after all enrichment completes
    const calculatedCell = document.querySelector(`.calculated-cell[data-item-id="${itemId}"]`);
    const diffCell = document.querySelector(`.diff-cell[data-item-id="${itemId}"]`);

    if (calculatedCell && diffCell) {
        const calcData = calculationCache.get(itemId);

        if (calcData) {
            // We have calculation data from backend
            if (calcData.cooStatus === 'missing') {
                calculatedCell.innerHTML = '<strong class="coo-missing">No COO set!</strong>';
                diffCell.innerHTML = '<strong class="coo-missing">No COO set!</strong>';
                diffCell.className = 'diff-cell coo-missing';
            } else {
                calculatedCell.textContent = '$' + calcData.calculatedCost.toFixed(2);
                const diffClass = calcData.diffStatus === 'ok' ? 'diff-ok' : 'diff-bad';
                const sign = calcData.diff >= 0 ? '+' : '';
                diffCell.textContent = sign + '$' + calcData.diff.toFixed(2);
                diffCell.className = `diff-cell ${diffClass}`;
            }
        }
        // If no calcData, leave cells as-is (spinners will be shown from initial render)
    }
}

// Image Carousel Functions
function openCarousel(offerId) {
    const images = carouselImages.get(offerId);
    if (!images || images.length === 0) {
        console.log('No images found for offer:', offerId);
        return;
    }

    currentCarouselImages = images;
    currentImageIndex = 0;
    updateCarouselImage();

    const modal = document.getElementById('imageCarousel');
    modal.classList.add('active');

    // Close on escape key
    document.addEventListener('keydown', handleCarouselKeys);

    // Close when clicking outside the modal content (on the backdrop)
    modal.addEventListener('click', handleModalBackdropClick);
}

function handleModalBackdropClick(e) {
    // Only close if clicking directly on the modal backdrop, not on its children
    if (e.target.id === 'imageCarousel') {
        closeCarousel();
    }
}

function closeCarousel() {
    const modal = document.getElementById('imageCarousel');
    modal.classList.remove('active');
    document.removeEventListener('keydown', handleCarouselKeys);
    modal.removeEventListener('click', handleModalBackdropClick);
}

function nextImage() {
    if (currentCarouselImages.length === 0) return;
    currentImageIndex = (currentImageIndex + 1) % currentCarouselImages.length;
    updateCarouselImage();
}

function prevImage() {
    if (currentCarouselImages.length === 0) return;
    currentImageIndex = (currentImageIndex - 1 + currentCarouselImages.length) % currentCarouselImages.length;
    updateCarouselImage();
}

function updateCarouselImage() {
    const image = document.getElementById('carouselImage');
    const indexSpan = document.getElementById('imageIndex');
    const totalSpan = document.getElementById('imageTotal');

    if (currentCarouselImages.length > 0) {
        image.src = currentCarouselImages[currentImageIndex];
        indexSpan.textContent = currentImageIndex + 1;
        totalSpan.textContent = currentCarouselImages.length;
    }

    // Hide navigation buttons if only one image
    const prevBtn = document.querySelector('.carousel-prev');
    const nextBtn = document.querySelector('.carousel-next');
    if (currentCarouselImages.length <= 1) {
        prevBtn.style.display = 'none';
        nextBtn.style.display = 'none';
    } else {
        prevBtn.style.display = 'block';
        nextBtn.style.display = 'block';
    }
}

function handleCarouselKeys(e) {
    if (e.key === 'Escape') {
        closeCarousel();
    } else if (e.key === 'ArrowRight') {
        nextImage();
    } else if (e.key === 'ArrowLeft') {
        prevImage();
    }
}

// Reference Data CRUD Functions

// Tariff Management
function openAddTariffModal() {
    document.getElementById('tariffModalTitle').textContent = 'Add Tariff Country';
    document.getElementById('tariffId').value = '';
    document.getElementById('tariffCountry').value = '';
    document.getElementById('tariffRate').value = '';
    document.getElementById('tariffNotes').value = '';
    document.getElementById('tariffModal').style.display = 'flex';
}

function editTariff(id) {
    const tariff = window.dbTariffs.find(t => t.id === id);
    if (!tariff) return;

    document.getElementById('tariffModalTitle').textContent = 'Edit Tariff Country';
    document.getElementById('tariffId').value = id;
    document.getElementById('tariffCountry').value = tariff.countryName;
    document.getElementById('tariffRate').value = (tariff.tariffRate * 100).toFixed(2);
    document.getElementById('tariffNotes').value = tariff.notes || '';
    document.getElementById('tariffModal').style.display = 'flex';
}

function closeTariffModal() {
    document.getElementById('tariffModal').style.display = 'none';
}

async function saveTariff(event) {
    event.preventDefault();

    const id = document.getElementById('tariffId').value;
    const data = {
        countryName: document.getElementById('tariffCountry').value.trim(),
        tariffRate: parseFloat(document.getElementById('tariffRate').value) / 100,
        notes: document.getElementById('tariffNotes').value.trim()
    };

    try {
        const url = id ? `/api/reference/tariffs/${id}` : '/api/reference/tariffs';
        const method = id ? 'PUT' : 'POST';

        const res = await secureFetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        if (!res.ok) {
            const error = await res.text();
            throw new Error(error);
        }

        closeTariffModal();
        await loadReferenceData();
        alert(`Tariff ${id ? 'updated' : 'created'} successfully`);
    } catch (err) {
        alert(`Error: ${err.message}`);
    }
}

async function deleteTariff(id) {
    const tariff = window.dbTariffs.find(t => t.id === id);
    if (!confirm(`Delete tariff for "${tariff.countryName}"? This will fail if any brands reference this country.`)) {
        return;
    }

    try {
        const res = await secureFetch(`/api/reference/tariffs/${id}`, {
            method: 'DELETE'
        });

        if (!res.ok) {
            const error = await res.text();
            throw new Error(error);
        }

        await loadReferenceData();
        alert('Tariff deleted successfully');
    } catch (err) {
        alert(`Error: ${err.message}`);
    }
}

// Brand Management
function openAddBrandModal() {
    document.getElementById('brandModalTitle').textContent = 'Add Brand';
    document.getElementById('brandId').value = '';
    document.getElementById('brandName').value = '';
    document.getElementById('brandNotes').value = '';

    // Populate COO dropdown with current tariff countries
    const select = document.getElementById('brandCOO');
    select.innerHTML = '<option value="">Select Country</option>' +
        window.dbTariffs.map(t => `<option value="${t.countryName}">${t.countryName}</option>`).join('');

    document.getElementById('brandModal').style.display = 'flex';
}

function editBrand(id) {
    const brand = window.dbBrands.find(b => b.id === id);
    if (!brand) return;

    document.getElementById('brandModalTitle').textContent = 'Edit Brand';
    document.getElementById('brandId').value = id;
    document.getElementById('brandName').value = brand.brandName;
    document.getElementById('brandNotes').value = brand.notes || '';

    // Populate COO dropdown
    const select = document.getElementById('brandCOO');
    select.innerHTML = '<option value="">Select Country</option>' +
        window.dbTariffs.map(t => `<option value="${t.countryName}" ${t.countryName === brand.primaryCoo ? 'selected' : ''}>${t.countryName}</option>`).join('');

    document.getElementById('brandModal').style.display = 'flex';
}

function closeBrandModal() {
    document.getElementById('brandModal').style.display = 'none';
}

async function saveBrand(event) {
    event.preventDefault();

    const id = document.getElementById('brandId').value;
    const data = {
        brandName: document.getElementById('brandName').value.trim(),
        primaryCoo: document.getElementById('brandCOO').value,
        notes: document.getElementById('brandNotes').value.trim()
    };

    if (!data.primaryCoo) {
        alert('Please select a country of origin');
        return;
    }

    try {
        const url = id ? `/api/reference/brands/${id}` : '/api/reference/brands';
        const method = id ? 'PUT' : 'POST';

        const res = await secureFetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        if (!res.ok) {
            const error = await res.text();
            throw new Error(error);
        }

        closeBrandModal();
        await loadReferenceData();
        alert(`Brand ${id ? 'updated' : 'created'} successfully`);
    } catch (err) {
        alert(`Error: ${err.message}`);
    }
}

async function deleteBrand(id) {
    const brand = window.dbBrands.find(b => b.id === id);
    if (!confirm(`Delete brand "${brand.brandName}"?`)) {
        return;
    }

    try {
        const res = await secureFetch(`/api/reference/brands/${id}`, {
            method: 'DELETE'
        });

        if (!res.ok) {
            const error = await res.text();
            throw new Error(error);
        }

        await loadReferenceData();
        alert('Brand deleted successfully');
    } catch (err) {
        alert(`Error: ${err.message}`);
    }
}
