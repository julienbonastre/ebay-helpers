-- Account tracking - identifies which eBay account data came from
-- Auto-created after OAuth, used to identify import source
CREATE TABLE IF NOT EXISTS accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_key TEXT NOT NULL UNIQUE,       -- e.g., "testuser_sandbox_EBAY_AU"
    display_name TEXT,                      -- e.g., "testuser Sandbox"
    ebay_user_id TEXT,                      -- eBay's immutable user ID (for deletion matching)
    ebay_username TEXT,                     -- eBay username from User API
    environment TEXT NOT NULL,              -- "production" or "sandbox"
    marketplace_id TEXT NOT NULL,           -- e.g., "EBAY_AU"
    last_export_at DATETIME,                -- When last export happened
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sync history
CREATE TABLE IF NOT EXISTS sync_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    sync_type TEXT NOT NULL,                -- "export" or "import"
    status TEXT NOT NULL,                   -- "success", "failed", "partial"
    items_synced INTEGER DEFAULT 0,
    error_message TEXT,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);

-- Fulfillment (shipping) policies - stores raw eBay FulfillmentPolicy JSON
CREATE TABLE IF NOT EXISTS fulfillment_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    policy_id TEXT NOT NULL,                -- For lookups
    name TEXT,                              -- For display
    marketplace_id TEXT,                    -- For filtering
    data TEXT NOT NULL,                     -- Full eBay FulfillmentPolicy JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    UNIQUE(account_id, policy_id)
);

-- Payment policies - stores raw eBay PaymentPolicy JSON
CREATE TABLE IF NOT EXISTS payment_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    policy_id TEXT NOT NULL,
    name TEXT,
    marketplace_id TEXT,
    data TEXT NOT NULL,                     -- Full eBay PaymentPolicy JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    UNIQUE(account_id, policy_id)
);

-- Return policies - stores raw eBay ReturnPolicy JSON
CREATE TABLE IF NOT EXISTS return_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    policy_id TEXT NOT NULL,
    name TEXT,
    marketplace_id TEXT,
    data TEXT NOT NULL,                     -- Full eBay ReturnPolicy JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    UNIQUE(account_id, policy_id)
);

-- Inventory items - stores raw eBay InventoryItem JSON
CREATE TABLE IF NOT EXISTS inventory_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    sku TEXT NOT NULL,
    title TEXT,                             -- For search/display
    brand TEXT,                             -- For filtering
    condition TEXT,                         -- For filtering
    data TEXT NOT NULL,                     -- Full eBay InventoryItem JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    UNIQUE(account_id, sku)
);

-- Offers (listings) - stores raw eBay Offer JSON
CREATE TABLE IF NOT EXISTS offers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    offer_id TEXT NOT NULL,
    sku TEXT NOT NULL,
    marketplace_id TEXT,
    listing_id TEXT,                        -- eBay listing ID for reference
    status TEXT,                            -- "PUBLISHED", "UNPUBLISHED" for filtering
    data TEXT NOT NULL,                     -- Full eBay Offer JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    UNIQUE(account_id, offer_id)
);

-- Brand to Country of Origin mappings (user-editable)
CREATE TABLE IF NOT EXISTS brand_coo_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    brand_name TEXT NOT NULL UNIQUE,        -- Brand name (e.g., "Free People")
    primary_coo TEXT NOT NULL,              -- Country of Origin (e.g., "China", "India")
    notes TEXT,                             -- Optional notes about the brand/supplier
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tariff rates by country (less frequently changed, government policy)
CREATE TABLE IF NOT EXISTS tariff_rates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    country_name TEXT NOT NULL UNIQUE,      -- e.g., "China", "Vietnam"
    tariff_rate REAL NOT NULL,              -- e.g., 0.20 for 20%
    notes TEXT,                             -- Context about the tariff
    effective_date DATE,                    -- When this rate became effective
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Marketplace account deletion notifications (eBay compliance requirement)
CREATE TABLE IF NOT EXISTS deletion_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    notification_id TEXT NOT NULL UNIQUE,   -- eBay's unique notification ID
    username TEXT NOT NULL,                 -- eBay username
    user_id TEXT,                           -- Immutable user identifier
    eias_token TEXT,                        -- Legacy token identifier
    event_date DATETIME NOT NULL,           -- When user requested deletion
    received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    processed BOOLEAN DEFAULT FALSE,        -- Whether we've handled the deletion
    processed_at DATETIME,                  -- When we processed it
    raw_payload TEXT NOT NULL               -- Full JSON payload for audit trail
);

-- Enriched item cache - stores brand and shipping data from GetItem API
-- Uses TTL to avoid redundant API calls (data rarely changes)
CREATE TABLE IF NOT EXISTS enriched_items (
    item_id TEXT PRIMARY KEY,               -- eBay Item ID (unique identifier)
    brand TEXT,                             -- Brand from GetItem API
    country_of_origin TEXT,                 -- Country of Origin from ItemSpecifics
    shipping_cost TEXT,                     -- US shipping cost
    shipping_currency TEXT,                 -- Shipping cost currency
    images TEXT,                            -- JSON array of full-size image URLs
    enriched_at DATETIME NOT NULL,          -- When this data was fetched (for TTL checking)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sessions - stores user session data (OAuth tokens)
-- Uses database storage to avoid cookie size limitations (eBay tokens are ~5KB)
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,            -- Random session identifier (stored in cookie)
    data TEXT NOT NULL,                     -- Session data (OAuth token JSON, encrypted)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL            -- When session expires (30 days default)
);

-- Postal zones - defines shipping zones with handling fees and tariff settings
CREATE TABLE IF NOT EXISTS postal_zones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id TEXT NOT NULL UNIQUE,           -- e.g., "1-New Zealand", "3-USA & Canada"
    zone_name TEXT NOT NULL,                -- Display name
    handling_fee_percent REAL DEFAULT 0.02, -- 2% handling fee
    has_tariffs BOOLEAN DEFAULT false,      -- Whether this zone has tariffs (USA only)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Postal rates - weight-based pricing for each zone
CREATE TABLE IF NOT EXISTS postal_rates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id TEXT NOT NULL,
    weight_band TEXT NOT NULL,              -- "XSmall", "Small", "Medium", "Large", "XLarge"
    max_weight_grams INTEGER NOT NULL,
    base_price_aud REAL NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (zone_id) REFERENCES postal_zones(zone_id),
    UNIQUE(zone_id, weight_band)
);

-- Discount bands - tier-based discounts for each zone
CREATE TABLE IF NOT EXISTS discount_bands (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id TEXT NOT NULL,
    band_level INTEGER NOT NULL,            -- 0-5
    discount_percent REAL NOT NULL,         -- 0.20 for 20%
    FOREIGN KEY (zone_id) REFERENCES postal_zones(zone_id),
    UNIQUE(zone_id, band_level)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_inventory_sku ON inventory_items(account_id, sku);
CREATE INDEX IF NOT EXISTS idx_offers_sku ON offers(account_id, sku);
CREATE INDEX IF NOT EXISTS idx_offers_status ON offers(account_id, status);
CREATE INDEX IF NOT EXISTS idx_sync_history_account ON sync_history(account_id, started_at);
CREATE INDEX IF NOT EXISTS idx_brand_coo_brand ON brand_coo_mappings(brand_name);
CREATE INDEX IF NOT EXISTS idx_tariff_country ON tariff_rates(country_name);
CREATE INDEX IF NOT EXISTS idx_enriched_items_at ON enriched_items(enriched_at);
CREATE INDEX IF NOT EXISTS idx_postal_rates_zone ON postal_rates(zone_id, weight_band);
