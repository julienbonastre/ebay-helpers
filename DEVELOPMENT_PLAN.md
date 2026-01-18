# eBay Helpers - Feature Development Plan

## Overview

This document outlines the implementation plan for 5 major features to be developed in parallel using git worktrees. Each feature has its own branch and can be worked on independently by separate Claude sessions.

---

## Git Workflow Setup

### Step 1: Create Plan Base Branch

First, commit this plan to a dedicated branch that all feature branches will derive from:

```bash
cd ebay-helpers
git checkout main
git pull origin main

# Create and switch to plan base branch
git checkout -b feature/development-plan-2026

# Copy plan file into repo (if using Claude Code plan mode)
# Plan will already be in your workspace

# Commit the plan
git add DEVELOPMENT_PLAN.md
git commit -m "Add development plan for 5 major features

Features planned:
- Global Settings (Settings tab with DB persistence)
- Reference Data CRUD (editable tariffs and brand COO)
- Multi-Zone Calculator (NZ, USA, UK/EU zones)
- Sync Import/Export (production to sandbox)
- Mobile Responsive (calculator view)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"

# Push plan branch
git push -u origin feature/development-plan-2026
```

### Step 2: Create Feature Worktrees (from plan branch)

```bash
# From the plan branch, create feature branches and worktrees
git worktree add ../ebay-helpers-settings feature/global-settings
git worktree add ../ebay-helpers-reference-crud feature/reference-data-crud
git worktree add ../ebay-helpers-multizone-calc feature/multizone-calculator
git worktree add ../ebay-helpers-sync feature/sync-import-export
git worktree add ../ebay-helpers-mobile feature/mobile-responsive

# Verify worktrees
git worktree list
```

Each worktree is a separate directory that can be opened in its own terminal/editor with an independent Claude session. All feature branches will have the `DEVELOPMENT_PLAN.md` file as reference.

---

## Feature 1: Global Settings (Settings Tab)

**Branch:** `feature/global-settings`
**Worktree:** `../ebay-helpers-settings`

### Summary
Add a new Settings tab with database-persisted user preferences, starting with AusPost Savings Tier selection.

### Database Changes

**New table in `internal/database/schema.sql`:**
```sql
CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,
    data_type TEXT DEFAULT 'string', -- 'string', 'int', 'float', 'bool', 'json'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_settings_key ON settings(key);

-- Seed initial settings
INSERT OR IGNORE INTO settings (key, value, description, data_type) VALUES
    ('auspost_savings_tier', '0', 'Current AusPost Savings Tier (0-5)', 'int'),
    ('auspost_api_enabled', 'false', 'Enable AusPost API integration (future)', 'bool'),
    ('auspost_api_key', '', 'AusPost API key (future)', 'string'),
    ('auspost_api_secret', '', 'AusPost API secret (future)', 'string');
```

### Backend Changes

**File: `internal/database/db.go`**
- Add `GetAllSettings() ([]Setting, error)`
- Add `GetSetting(key string) (*Setting, error)`
- Add `UpdateSetting(key, value string) error`
- Add `Setting` struct with Key, Value, Description, DataType, UpdatedAt

**File: `internal/handlers/handlers.go`**
- Add `GET /api/settings` - Return all settings
- Add `PUT /api/settings/:key` - Update single setting
- Add `SettingsHandler` struct/methods

**File: `cmd/server/main.go`**
- Register new routes

### Frontend Changes

**File: `cmd/server/web/index.html`**
- Add Settings tab button in nav (5th tab)
- Add Settings tab content section with form

**File: `cmd/server/web/app.js`**
- Add `loadSettingsTab()` function
- Add `updateSetting(key, value)` function
- Add Settings form rendering with:
  - AusPost Savings Tier dropdown (0-5)
  - Disabled checkbox for "Use AusPost API" (future feature indicator)
  - Disabled inputs for API credentials (greyed out, future feature)

### UI Design
```
┌─────────────────────────────────────────────────┐
│ Settings                                        │
├─────────────────────────────────────────────────┤
│ AusPost Configuration                           │
│ ┌─────────────────────────────────────────────┐ │
│ │ Current Savings Tier: [Dropdown 0-5 ▼]      │ │
│ │                                             │ │
│ │ ☐ Use AusPost API (Coming Soon)  [disabled] │ │
│ │   API Key: [________________]    [disabled] │ │
│ │   API Secret: [______________]   [disabled] │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ [Save Settings]                                 │
└─────────────────────────────────────────────────┘
```

### Testing
1. Start server, navigate to Settings tab
2. Change savings tier, verify persists across restarts
3. Verify calculator uses saved tier as default

---

## Feature 2: Reference Data CRUD

**Branch:** `feature/reference-data-crud`
**Worktree:** `../ebay-helpers-reference-crud`

### Summary
Transform the read-only Reference Data tab into an editable interface with CRUD operations for Tariff Countries and Brand COO mappings.

### Database Changes

**File: `internal/database/schema.sql`**
- Ensure `tariff_rates` table has proper structure (already exists)
- Ensure `brand_coo_mappings` table has proper structure (already exists)
- Add foreign key awareness (Brand COO must reference valid tariff country)

### Backend Changes

**File: `internal/database/db.go`**
Add/verify these functions:
- `GetAllTariffRates() ([]TariffRate, error)` ✓ exists
- `CreateTariffRate(countryName string, rate float64, notes string) error`
- `UpdateTariffRate(id int, countryName string, rate float64, notes string) error`
- `DeleteTariffRate(id int) error`
- `GetAllBrandCOOMappings() ([]BrandCOOMapping, error)` ✓ exists
- `CreateBrandCOOMapping(...)` ✓ exists
- `UpdateBrandCOOMapping(...)` ✓ exists
- `DeleteBrandCOOMapping(...)` ✓ exists

**File: `internal/handlers/handlers.go`**
New endpoints:
- `GET /api/reference/tariffs` - List all tariff rates
- `POST /api/reference/tariffs` - Create new tariff rate
- `PUT /api/reference/tariffs/:id` - Update tariff rate
- `DELETE /api/reference/tariffs/:id` - Delete tariff rate
- `GET /api/reference/brands` - List all brand mappings (from DB)
- `POST /api/reference/brands` - Create new brand mapping
- `PUT /api/reference/brands/:id` - Update brand mapping
- `DELETE /api/reference/brands/:id` - Delete brand mapping

**File: `internal/calculator/calculator.go`**
- Modify to read from database instead of hardcoded `data.go`
- Add `RefreshReferenceData()` to reload from DB
- Keep `data.go` as fallback/seed data

### Frontend Changes

**File: `cmd/server/web/index.html`**
- Restructure Reference Data tab with editable tables
- Add modal for add/edit forms
- Add delete confirmation dialogue

**File: `cmd/server/web/app.js`**
New functions:
- `loadReferenceDataFromDB()` - Fetch from new endpoints
- `renderTariffTable(tariffs, editable=true)`
- `renderBrandTable(brands, tariffs)` - Brands with COO dropdown
- `openAddTariffModal()` / `openEditTariffModal(id)`
- `openAddBrandModal()` / `openEditBrandModal(id)`
- `saveTariff(data)` / `deleteTariff(id)`
- `saveBrand(data)` / `deleteBrand(id)`
- `validateBrandCOO(coo)` - Ensure COO exists in tariff countries

### UI Design
```
┌─────────────────────────────────────────────────────────────┐
│ Reference Data                                              │
├─────────────────────────────────────────────────────────────┤
│ Tariff Rates by Country                    [+ Add Country]  │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Country      │ Rate (%) │ Notes        │ Actions       │ │
│ ├──────────────┼──────────┼──────────────┼───────────────┤ │
│ │ China        │ 20%      │ IEEPA tariff │ [Edit][Delete]│ │
│ │ India        │ 50%      │ Highest rate │ [Edit][Delete]│ │
│ │ Vietnam      │ 20%      │              │ [Edit][Delete]│ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ Brand → Country of Origin                    [+ Add Brand]  │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Brand        │ Primary COO │ Rate │ Notes │ Actions    │ │
│ ├──────────────┼─────────────┼──────┼───────┼────────────┤ │
│ │ Free People  │ [China ▼]   │ 20%  │       │ [Edit][Del]│ │
│ │ Aje          │ [China ▼]   │ 20%  │       │ [Edit][Del]│ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ Weight Bands (Read Only - AusPost Official Rates)           │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ ... existing weight bands table - no edit buttons ...   │ │
│ │ (Future: will be auto-updated via AusPost API)          │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

**Decision:** Weight Bands remain read-only. These are AusPost official rates - manual editing could cause incorrect calculations. Future AusPost API integration will auto-update these.

### Key Behaviours
- Brand COO dropdown only shows countries that exist in Tariff table
- Adding new tariff country immediately makes it available in Brand COO dropdown
- Cannot delete a tariff country if brands reference it (show error)
- Real-time validation and feedback

### Testing
1. Add new tariff country (e.g., "Thailand" at 15%)
2. Create new brand mapping with Thailand as COO
3. Verify calculator uses new data
4. Try deleting Thailand - should fail if brand references it
5. Delete brand, then delete Thailand - should succeed

---

## Feature 3: Multi-Zone Calculator

**Branch:** `feature/multizone-calculator`
**Worktree:** `../ebay-helpers-multizone-calc`

### Summary
Extend the calculator to support all shipping zones from the Excel spreadsheet, displaying costs for all destinations simultaneously.

### Reference Data (from TariffAndPostalCalculator.xlsx)

**Zone 1: New Zealand**
| Weight Band | Max Weight | Base Price |
|-------------|------------|------------|
| XSmall      | 250g       | $15.80     |
| Small       | 500g       | $20.50     |
| Medium      | 1kg        | $29.90     |
| Large       | 1.5kg      | $39.30     |
| XLarge      | 2kg        | $48.70     |

**Zone 3: USA & Canada** (existing)
| Weight Band | Max Weight | Base Price |
|-------------|------------|------------|
| XSmall      | 250g       | $22.30     |
| Small       | 500g       | $29.00     |
| Medium      | 1kg        | $42.20     |
| Large       | 1.5kg      | $55.55     |
| XLarge      | 2kg        | $68.85     |

**Zone 4: UK, Ireland, Major EU**
| Weight Band | Max Weight | Base Price |
|-------------|------------|------------|
| XSmall      | 250g       | $22.30     |
| Small       | 500g       | $29.00     |
| Medium      | 1kg        | $42.20     |
| Large       | 1.5kg      | $55.55     |
| XLarge      | 2kg        | $68.85     |

*Note: Zone 4 prices same as Zone 3 per spreadsheet*

### Database Changes

**File: `internal/database/schema.sql`**
```sql
CREATE TABLE IF NOT EXISTS postal_zones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id TEXT NOT NULL UNIQUE,
    zone_name TEXT NOT NULL,
    handling_fee_percent REAL DEFAULT 0.02,
    has_tariffs BOOLEAN DEFAULT false,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS postal_rates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id TEXT NOT NULL,
    weight_band TEXT NOT NULL,
    max_weight_grams INTEGER NOT NULL,
    base_price_aud REAL NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (zone_id) REFERENCES postal_zones(zone_id),
    UNIQUE(zone_id, weight_band)
);

CREATE TABLE IF NOT EXISTS discount_bands (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id TEXT NOT NULL,
    band_level INTEGER NOT NULL,
    discount_percent REAL NOT NULL,
    FOREIGN KEY (zone_id) REFERENCES postal_zones(zone_id),
    UNIQUE(zone_id, band_level)
);
```

### Backend Changes

**File: `internal/calculator/data.go`**
- Add all zones to `PostalZones` map
- Add zone-specific data structures

**File: `internal/calculator/calculator.go`**
- Add `CalculateAllZones(params)` function returning costs for all zones
- Modify to handle zone-specific tariff application (only USA has tariffs)
- Add `MultiZoneResult` struct

**File: `internal/handlers/handlers.go`**
- Add `POST /api/calculate/all-zones` endpoint
- Return array of results for each zone

### Frontend Changes

**File: `cmd/server/web/app.js`**
- Modify calculator to call `/api/calculate/all-zones`
- Render results grid showing all destinations
- Highlight USA result (has tariffs)
- Add zone selection for single-zone mode (optional)

### UI Design

**Decision:** All zones display simultaneously - one calculation shows all destinations at once. This is ideal for quick eBay listing setup where you need to see all postage costs together.

```
┌─────────────────────────────────────────────────────────────────┐
│ Postage Calculator                                              │
├─────────────────────────────────────────────────────────────────┤
│ Item Value (AUD): [________]  Weight Band: [Medium ▼]           │
│ Brand: [Free People ▼]        COO Override: [________]          │
│ Discount Band: [3 ▼]          ☑ Include Extra Cover             │
│                                                                 │
│ [Calculate All Zones]                                           │
├─────────────────────────────────────────────────────────────────┤
│ Results                                                         │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ Zone 1: New Zealand                                         │ │
│ │ Base: $29.90 | Extra Cover: $4.80 | Total: $34.70          │ │
│ ├─────────────────────────────────────────────────────────────┤ │
│ │ Zone 3: USA & Canada ⚠️ (Tariffs Apply)                      │ │
│ │ Base: $42.20 | Extra Cover: $4.80 | Tariff: $50.00         │ │
│ │ Zonos: $6.69 | Total: $103.69                              │ │
│ ├─────────────────────────────────────────────────────────────┤ │
│ │ Zone 4: UK, Ireland & EU                                    │ │
│ │ Base: $42.20 | Extra Cover: $4.80 | Total: $47.00          │ │
│ └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Testing
1. Calculate for $250 Medium weight item
2. Verify NZ price is cheapest (no tariffs, lower base)
3. Verify USA has highest total (tariffs + Zonos)
4. Verify UK/EU matches USA base but no tariffs
5. Test all weight bands across zones

---

## Feature 4: Sync Import/Export (Complete Implementation)

**Branch:** `feature/sync-import-export`
**Worktree:** `../ebay-helpers-sync`

### Summary
Complete the sync feature to enable full export from production and import to sandbox, including the missing eBay API create methods.

### Current State
- **Export:** ✅ Complete (policies, inventory items, offers)
- **Import:** ❌ Placeholder only (no eBay create methods)

### Backend Changes

**File: `internal/ebay/client.go`**
Add new methods:
```go
// Inventory Management
func (c *Client) CreateInventoryItem(sku string, item InventoryItem) error
func (c *Client) UpdateInventoryItem(sku string, item InventoryItem) error
func (c *Client) DeleteInventoryItem(sku string) error

// Offer Management
func (c *Client) CreateOffer(offer Offer) (*Offer, error)
func (c *Client) UpdateOffer(offerID string, offer Offer) error
func (c *Client) DeleteOffer(offerID string) error
func (c *Client) PublishOffer(offerID string) (*PublishResponse, error)

// Policy Management
func (c *Client) CreateFulfillmentPolicy(policy FulfillmentPolicy) (*FulfillmentPolicy, error)
func (c *Client) CreatePaymentPolicy(policy PaymentPolicy) (*PaymentPolicy, error)
func (c *Client) CreateReturnPolicy(policy ReturnPolicy) (*ReturnPolicy, error)
```

**eBay API Endpoints:**
- Inventory: `PUT /sell/inventory/v1/inventory_item/{sku}`
- Offer: `POST /sell/inventory/v1/offer`
- Fulfillment Policy: `POST /sell/account/v1/fulfillment_policy`
- Payment Policy: `POST /sell/account/v1/payment_policy`
- Return Policy: `POST /sell/account/v1/return_policy`

**File: `internal/sync/sync.go`**
Complete `ImportToEbay()`:
1. Import policies first (create if not exists, get policy IDs)
2. Import inventory items (create/update by SKU)
3. Import offers (link to inventory items + policies)
4. Handle ID remapping (source account IDs → target account IDs)

**File: `internal/handlers/handlers.go`**
- Add progress callback support for long-running imports
- Add `/api/sync/status/:syncId` for polling progress

### Policy Conflict Handling

**Decision:** Prompt user before import with options. Default is "Skip existing" but allow "Clean slate" mode since sandbox data isn't precious - typically users want to replicate prod exactly.

**Import Options Dialog:**
- **Skip existing (default)** - If policy with same name exists, use it
- **Clean slate** - Delete all existing policies/items in target, then import fresh

### Data Transformation Challenges

**Policy ID Remapping:**
```
Source Account                    Target Account
------------------               ------------------
FulfillmentPolicy ID: 12345  →   FulfillmentPolicy ID: 67890
Offer references: 12345      →   Update to: 67890
```

**Implementation:**
1. Export stores raw eBay JSON
2. If "Clean slate" mode, delete existing target data first
3. Import creates new policies, receives new IDs
4. Build mapping table: `{sourcePolicyId: targetPolicyId}`
5. When importing offers, replace policy IDs using mapping

### Frontend Changes

**File: `cmd/server/web/app.js`**
- Add import progress indicator (items imported / total)
- Add import preview showing what will be created
- Add "dry run" mode to preview without creating
- Improve error handling for partial imports

### UI Enhancements
```
┌─────────────────────────────────────────────────────────────┐
│ Import from Production                                      │
├─────────────────────────────────────────────────────────────┤
│ Source Account: [production_user ▼]                         │
│                                                             │
│ Import Preview:                                             │
│ • 3 Fulfillment Policies                                    │
│ • 2 Payment Policies                                        │
│ • 2 Return Policies                                         │
│ • 156 Inventory Items                                       │
│ • 142 Active Offers                                         │
│                                                             │
│ Conflict Handling:                                          │
│ ○ Skip existing (use target's policies) [default]          │
│ ○ Clean slate (delete target data, import fresh)           │
│                                                             │
│ ☐ Dry Run (preview only)                                    │
│                                                             │
│ [Start Import]                                              │
│                                                             │
│ Progress: [████████████░░░░░░░░] 65/156 items              │
│ Status: Creating inventory item SKU-0065...                 │
└─────────────────────────────────────────────────────────────┘
```

### Testing
1. Connect to production account, run export
2. Switch to sandbox account (restart with sandbox creds)
3. Run import from production export
4. Verify all policies created
5. Verify inventory items created
6. Verify offers created and linked correctly

---

## Feature 5: Mobile Responsive Calculator

**Branch:** `feature/mobile-responsive`
**Worktree:** `../ebay-helpers-mobile`

### Summary
Make the calculator view mobile-friendly with proper portrait orientation support.

### CSS Changes

**File: `cmd/server/web/index.html`**
Add mobile breakpoints:
```css
/* Mobile-first calculator styles */
@media (max-width: 768px) {
    .calc-form {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .calc-form .form-group {
        width: 100%;
    }

    .calc-form input,
    .calc-form select {
        font-size: 16px; /* Prevents iOS zoom on focus */
        padding: 0.75rem;
    }

    .calc-results {
        display: flex;
        flex-direction: column;
        gap: 0.75rem;
    }

    .zone-result {
        padding: 1rem;
        border-radius: 0.5rem;
        background: var(--card);
    }

    .zone-result .total {
        font-size: 1.5rem;
        font-weight: bold;
    }

    /* Stack breakdown vertically on mobile */
    .result-breakdown {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
        font-size: 0.875rem;
    }

    /* Full-width calculate button */
    .calc-form button[type="submit"] {
        width: 100%;
        padding: 1rem;
        font-size: 1.1rem;
    }
}

@media (max-width: 480px) {
    /* Extra small screens */
    .container {
        padding: 0.5rem;
    }

    .tab-nav {
        flex-wrap: wrap;
        gap: 0.25rem;
    }

    .tab-nav button {
        flex: 1 1 auto;
        min-width: 80px;
        font-size: 0.8rem;
        padding: 0.5rem;
    }
}
```

### JavaScript Changes

**File: `cmd/server/web/app.js`**
- Add viewport detection: `isMobile()` helper
- Conditionally render compact vs expanded result views
- Add touch-friendly interactions (larger tap targets)

### Mobile-Optimised Calculator Layout
```
┌──────────────────────┐
│ Postage Calculator   │
├──────────────────────┤
│ Item Value (AUD)     │
│ ┌──────────────────┐ │
│ │ $250             │ │
│ └──────────────────┘ │
│                      │
│ Weight Band          │
│ ┌──────────────────┐ │
│ │ Medium (500-1kg) │ │
│ └──────────────────┘ │
│                      │
│ Brand                │
│ ┌──────────────────┐ │
│ │ Free People      │ │
│ └──────────────────┘ │
│                      │
│ Discount Band        │
│ ┌──────────────────┐ │
│ │ Band 3 (20%)     │ │
│ └──────────────────┘ │
│                      │
│ ☑ Include Extra Cover│
│                      │
│ ┌──────────────────┐ │
│ │   CALCULATE      │ │
│ └──────────────────┘ │
├──────────────────────┤
│ ▼ New Zealand        │
│   Total: $34.70      │
├──────────────────────┤
│ ▼ USA & Canada ⚠️    │
│   Total: $103.69     │
│   (incl. tariffs)    │
├──────────────────────┤
│ ▼ UK & Ireland       │
│   Total: $47.00      │
└──────────────────────┘
```

### Touch Interactions
- Collapsible zone result cards (tap to expand breakdown)
- Swipe between zones (optional enhancement)
- Pull-to-refresh for recalculation (optional)

### Testing
1. Open in Chrome DevTools mobile emulation
2. Test on actual iOS/Android devices
3. Verify no horizontal scroll
4. Verify inputs don't zoom on focus (iOS)
5. Verify tap targets are 44px+ minimum
6. Test landscape orientation

---

## Implementation Order Recommendation

These features can be developed in parallel, but if sequencing is needed:

1. **Feature 1: Global Settings** - Foundation for other features (settings storage pattern)
2. **Feature 2: Reference Data CRUD** - Enables dynamic data management
3. **Feature 3: Multi-Zone Calculator** - Depends on reference data patterns
4. **Feature 5: Mobile Responsive** - Can be done anytime, independent
5. **Feature 4: Sync** - Most complex, benefits from stable codebase

---

## Verification Steps

After each feature is complete:

1. **Build:** `go build -o /tmp/ebay-helpers ./cmd/server`
2. **Test:** Run server, manually test feature
3. **Merge:** Create PR to main, review, merge
4. **Update worktrees:** `git worktree list` and update main in each

---

## Files Summary

### Feature 1: Global Settings
- `internal/database/schema.sql` - Add settings table
- `internal/database/db.go` - Add settings CRUD
- `internal/handlers/handlers.go` - Add settings endpoints
- `cmd/server/main.go` - Register routes
- `cmd/server/web/index.html` - Add Settings tab
- `cmd/server/web/app.js` - Add settings functions

### Feature 2: Reference Data CRUD
- `internal/database/db.go` - Add/verify CRUD functions
- `internal/handlers/handlers.go` - Add reference endpoints
- `internal/calculator/calculator.go` - Read from DB
- `cmd/server/web/index.html` - Add edit UI
- `cmd/server/web/app.js` - Add CRUD functions

### Feature 3: Multi-Zone Calculator
- `internal/database/schema.sql` - Add zones tables
- `internal/calculator/data.go` - Add zone data
- `internal/calculator/calculator.go` - Add multi-zone calc
- `internal/handlers/handlers.go` - Add all-zones endpoint
- `cmd/server/web/app.js` - Update calculator UI

### Feature 4: Sync Import/Export
- `internal/ebay/client.go` - Add create/update methods
- `internal/sync/sync.go` - Complete import logic
- `internal/handlers/handlers.go` - Add progress endpoint
- `cmd/server/web/app.js` - Add progress UI

### Feature 5: Mobile Responsive
- `cmd/server/web/index.html` - Add media queries
- `cmd/server/web/app.js` - Add mobile helpers
