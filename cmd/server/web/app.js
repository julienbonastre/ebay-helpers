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
let listings = [];
let currentPage = 0;
let pageSize = 25;
let totalListings = 0;
let selectedItems = new Set();
let brands = [];
let tariffCountries = [];
let weightBands = [];

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    initTheme();
    setupTabs();
    await checkAuthStatus();
    await loadReferenceData();

    // Check for auth success redirect
    const params = new URLSearchParams(window.location.search);
    if (params.get('auth') === 'success') {
        window.history.replaceState({}, '', '/');
        await checkAuthStatus();
        showTab('listings');
        loadListings();
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

function showTab(tabId) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    document.querySelector(`.tab[data-tab="${tabId}"]`).classList.add('active');
    document.getElementById(tabId).classList.add('active');
}

// Auth
let isConfigured = false;

async function checkAuthStatus() {
    try {
        const res = await fetch('/api/auth/status');
        const data = await res.json();
        isAuthenticated = data.authenticated;
        isConfigured = data.configured;
        updateAuthUI();
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
}

async function handleAuth() {
    try {
        const res = await fetch('/api/auth/url');
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
                const statusRes = await fetch('/api/auth/status');
                const statusData = await statusRes.json();

                if (statusData.authenticated) {
                    clearInterval(pollInterval);
                    if (authWindow && !authWindow.closed) {
                        authWindow.close();
                    }
                    await checkAuthStatus();
                    alert('Successfully connected to eBay!');
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
        const [brandsRes, countriesRes, bandsRes] = await Promise.all([
            fetch('/api/brands'),
            fetch('/api/tariff-countries'),
            fetch('/api/weight-bands')
        ]);

        const brandsData = await brandsRes.json();
        const countriesData = await countriesRes.json();
        const bandsData = await bandsRes.json();

        brands = brandsData.brands || [];
        tariffCountries = countriesData.countries || [];
        weightBands = bandsData.weightBands || [];

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
    // Tariff table
    const tariffBody = document.querySelector('#tariffTable tbody');
    tariffBody.innerHTML = tariffCountries.map(c =>
        `<tr><td>${c.country}</td><td>${c.ratePercent}%</td></tr>`
    ).join('');

    // Weight bands table
    const weightBody = document.querySelector('#weightTable tbody');
    weightBody.innerHTML = weightBands.map(b =>
        `<tr><td>${b.key}</td><td>${b.label.replace(b.key + ' ', '')}</td><td>$${b.basePrice.toFixed(2)}</td></tr>`
    ).join('');

    // Brand table (we need to fetch COO for each)
    const brandBody = document.querySelector('#brandTable tbody');
    // For simplicity, just show brands - COO comes from calculator
    brandBody.innerHTML = brands.map(b => {
        const country = getBrandCOO(b);
        const tariff = tariffCountries.find(c => c.country === country);
        return `<tr><td>${b}</td><td>${country}</td><td>${tariff ? tariff.ratePercent + '%' : '-'}</td></tr>`;
    }).join('');
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
        const res = await fetch('/api/calculate', {
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

// Listings
async function loadListings() {
    if (!isAuthenticated) {
        document.getElementById('listingsLoading').style.display = 'none';
        document.getElementById('listingsEmpty').style.display = 'block';
        document.getElementById('listingsEmpty').textContent = 'Please connect to eBay first.';
        return;
    }

    document.getElementById('listingsLoading').style.display = 'flex';
    document.getElementById('listingsTable').style.display = 'none';
    document.getElementById('listingsEmpty').style.display = 'none';

    try {
        const offset = currentPage * pageSize;
        const res = await fetch(`/api/offers?limit=${pageSize}&offset=${offset}`);
        const data = await res.json();

        if (data.error) {
            throw new Error(data.error);
        }

        listings = data.offers || [];
        totalListings = data.total || 0;

        renderListings();
    } catch (err) {
        console.error('Failed to load listings:', err);
        document.getElementById('listingsLoading').style.display = 'none';
        document.getElementById('listingsEmpty').style.display = 'block';
        document.getElementById('listingsEmpty').textContent = 'Failed to load: ' + err.message;
    }
}

async function renderListings() {
    document.getElementById('listingsLoading').style.display = 'none';

    if (listings.length === 0) {
        document.getElementById('listingsEmpty').style.display = 'block';
        document.getElementById('listingsEmpty').textContent = 'No active listings found.';
        return;
    }

    document.getElementById('listingsTable').style.display = 'block';
    document.getElementById('listingsPagination').style.display = 'flex';

    const tbody = document.getElementById('listingsBody');
    const rows = await Promise.all(listings.map(async (offer) => {
        const price = offer.pricingSummary?.price?.value || '0';
        const sku = offer.sku || '-';

        // Get current shipping override for US
        let currentUSPostage = '-';
        const overrides = offer.listingPolicies?.shippingCostOverrides || [];
        const intlOverride = overrides.find(o => o.shippingServiceType === 'INTERNATIONAL');
        if (intlOverride?.shippingCost?.value) {
            currentUSPostage = '$' + parseFloat(intlOverride.shippingCost.value).toFixed(2);
        }

        // Calculate expected postage (simplified - uses Medium weight, no extra cover)
        let calculated = '-';
        let diff = '-';
        let diffClass = '';

        try {
            const calcRes = await fetch('/api/calculate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    itemValueAUD: parseFloat(price) || 0,
                    weightBand: 'Medium', // Default - would need item weight data
                    brandName: 'Spell', // Default - would need brand detection
                    includeExtraCover: parseFloat(price) >= 250,
                    discountBand: 0
                })
            });
            const calcData = await calcRes.json();
            if (calcData.totalShipping) {
                calculated = '$' + calcData.totalShipping.toFixed(2);

                if (currentUSPostage !== '-') {
                    const current = parseFloat(currentUSPostage.replace('$', ''));
                    const expected = calcData.totalShipping;
                    const diffVal = current - expected;
                    diff = (diffVal >= 0 ? '+' : '') + diffVal.toFixed(2);

                    if (Math.abs(diffVal) < 2) diffClass = 'diff-ok';
                    else if (Math.abs(diffVal) < 10) diffClass = 'diff-warn';
                    else diffClass = 'diff-bad';
                }
            }
        } catch (e) {
            console.error('Calc error:', e);
        }

        return `
            <tr data-offer-id="${offer.offerId}">
                <td class="checkbox-cell">
                    <input type="checkbox" onchange="toggleSelect('${offer.offerId}')"
                           ${selectedItems.has(offer.offerId) ? 'checked' : ''}>
                </td>
                <td><img src="https://via.placeholder.com/50" class="thumbnail" alt=""></td>
                <td>${offer.listingDescription?.substring(0, 50) || sku}...</td>
                <td>${sku}</td>
                <td class="price">$${parseFloat(price).toFixed(2)}</td>
                <td>-</td>
                <td>Medium</td>
                <td>${currentUSPostage}</td>
                <td>${calculated}</td>
                <td class="${diffClass}">${diff}</td>
                <td>
                    <button class="btn btn-sm btn-secondary" onclick="editItem('${offer.offerId}')">Edit</button>
                </td>
            </tr>
        `;
    }));

    tbody.innerHTML = rows.join('');

    // Update pagination
    const start = currentPage * pageSize + 1;
    const end = Math.min(start + listings.length - 1, totalListings);
    document.getElementById('pageStart').textContent = start;
    document.getElementById('pageEnd').textContent = end;
    document.getElementById('pageTotal').textContent = totalListings;

    document.getElementById('prevPage').disabled = currentPage === 0;
    document.getElementById('nextPage').disabled = end >= totalListings;
}

function prevPage() {
    if (currentPage > 0) {
        currentPage--;
        loadListings();
    }
}

function nextPage() {
    currentPage++;
    loadListings();
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
