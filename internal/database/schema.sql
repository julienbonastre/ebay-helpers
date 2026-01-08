-- Account profiles for multi-account management
CREATE TABLE IF NOT EXISTS accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,              -- e.g., "La Troverie Prod", "La Troverie Sandbox"
    environment TEXT NOT NULL,              -- "production" or "sandbox"
    marketplace_id TEXT NOT NULL,           -- e.g., "EBAY_AU"
    client_id TEXT,                         -- OAuth credentials (optional, can use env vars)
    client_secret TEXT,
    redirect_uri TEXT,
    is_active BOOLEAN DEFAULT 0,            -- Currently selected account
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

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_accounts_active ON accounts(is_active);
CREATE INDEX IF NOT EXISTS idx_inventory_sku ON inventory_items(account_id, sku);
CREATE INDEX IF NOT EXISTS idx_offers_sku ON offers(account_id, sku);
CREATE INDEX IF NOT EXISTS idx_offers_status ON offers(account_id, status);
CREATE INDEX IF NOT EXISTS idx_sync_history_account ON sync_history(account_id, started_at);
