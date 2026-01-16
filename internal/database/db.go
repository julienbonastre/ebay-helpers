package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps the SQLite database
type DB struct {
	*sql.DB
}

// Account represents an eBay account identifier for data tracking
type Account struct {
	ID            int64      `json:"id"`
	AccountKey    string     `json:"accountKey"`    // Unique key: "username_env_marketplace"
	DisplayName   string     `json:"displayName"`   // Human readable: "username Production"
	EbayUserID    string     `json:"ebayUserId"`    // eBay's immutable user ID
	EbayUsername  string     `json:"ebayUsername"`  // eBay username
	Environment   string     `json:"environment"`   // "production" or "sandbox"
	MarketplaceID string     `json:"marketplaceId"` // "EBAY_AU"
	LastExportAt  *time.Time `json:"lastExportAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// SyncHistory represents a sync operation record
type SyncHistory struct {
	ID           int64      `json:"id"`
	AccountID    int64      `json:"accountId"`
	SyncType     string     `json:"syncType"` // "export" or "import"
	Status       string     `json:"status"`   // "success", "failed", "partial"
	ItemsSynced  int        `json:"itemsSynced"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	StartedAt    time.Time  `json:"startedAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

// Open opens or creates the database
func Open(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Initialize schema
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &DB{db}, nil
}

// GetOrCreateAccount gets an account by key or creates it if it doesn't exist
func (db *DB) GetOrCreateAccount(accountKey, displayName, environment, marketplaceID string) (*Account, error) {
	// Try to get existing
	var acc Account
	err := db.QueryRow(`
		SELECT id, account_key, display_name, COALESCE(ebay_user_id, ''), COALESCE(ebay_username, ''),
		       environment, marketplace_id, last_export_at, created_at, updated_at
		FROM accounts
		WHERE account_key = ?
	`, accountKey).Scan(&acc.ID, &acc.AccountKey, &acc.DisplayName, &acc.EbayUserID, &acc.EbayUsername,
		&acc.Environment, &acc.MarketplaceID, &acc.LastExportAt, &acc.CreatedAt, &acc.UpdatedAt)

	if err == nil {
		return &acc, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new
	result, err := db.Exec(`
		INSERT INTO accounts (account_key, display_name, environment, marketplace_id)
		VALUES (?, ?, ?, ?)
	`, accountKey, displayName, environment, marketplaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	acc.ID = id
	acc.AccountKey = accountKey
	acc.DisplayName = displayName
	acc.Environment = environment
	acc.MarketplaceID = marketplaceID
	acc.CreatedAt = time.Now()
	acc.UpdatedAt = time.Now()

	return &acc, nil
}

// UpdateAccountWithEbayInfo updates an account with eBay user information after OAuth
func (db *DB) UpdateAccountWithEbayInfo(accountID int64, ebayUserID, ebayUsername string) error {
	_, err := db.Exec(`
		UPDATE accounts
		SET ebay_user_id = ?, ebay_username = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, ebayUserID, ebayUsername, accountID)
	return err
}

// GetOrCreateAccountFromEbay gets or creates an account using eBay user info
func (db *DB) GetOrCreateAccountFromEbay(ebayUserID, ebayUsername, environment, marketplaceID string) (*Account, error) {
	accountKey := ebayUsername + "_" + environment + "_" + marketplaceID
	displayName := ebayUsername
	if environment == "production" {
		displayName += " Production"
	} else {
		displayName += " Sandbox"
	}

	// Try to get existing by eBay user ID + environment
	var acc Account
	err := db.QueryRow(`
		SELECT id, account_key, display_name, COALESCE(ebay_user_id, ''), COALESCE(ebay_username, ''),
		       environment, marketplace_id, last_export_at, created_at, updated_at
		FROM accounts
		WHERE ebay_user_id = ? AND environment = ? AND marketplace_id = ?
	`, ebayUserID, environment, marketplaceID).Scan(&acc.ID, &acc.AccountKey, &acc.DisplayName,
		&acc.EbayUserID, &acc.EbayUsername, &acc.Environment, &acc.MarketplaceID,
		&acc.LastExportAt, &acc.CreatedAt, &acc.UpdatedAt)

	if err == nil {
		// Update username if it changed
		if acc.EbayUsername != ebayUsername {
			_ = db.UpdateAccountWithEbayInfo(acc.ID, ebayUserID, ebayUsername)
			acc.EbayUsername = ebayUsername
		}
		return &acc, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new or update if account_key already exists
	result, err := db.Exec(`
		INSERT INTO accounts (account_key, display_name, ebay_user_id, ebay_username, environment, marketplace_id)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_key) DO UPDATE SET
			display_name = excluded.display_name,
			ebay_user_id = excluded.ebay_user_id,
			ebay_username = excluded.ebay_username,
			environment = excluded.environment,
			marketplace_id = excluded.marketplace_id,
			updated_at = CURRENT_TIMESTAMP
	`, accountKey, displayName, ebayUserID, ebayUsername, environment, marketplaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update account: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	acc.ID = id
	acc.AccountKey = accountKey
	acc.DisplayName = displayName
	acc.EbayUserID = ebayUserID
	acc.EbayUsername = ebayUsername
	acc.Environment = environment
	acc.MarketplaceID = marketplaceID
	acc.CreatedAt = time.Now()
	acc.UpdatedAt = time.Now()

	return &acc, nil
}

// UpdateLastExport updates the last export timestamp for an account
func (db *DB) UpdateLastExport(accountID int64) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE accounts
		SET last_export_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, now, accountID)
	return err
}

// GetAccounts returns all tracked accounts (that have exported data)
func (db *DB) GetAccounts() ([]Account, error) {
	rows, err := db.Query(`
		SELECT id, account_key, display_name, COALESCE(ebay_user_id, ''), COALESCE(ebay_username, ''),
		       environment, marketplace_id, last_export_at, created_at, updated_at
		FROM accounts
		ORDER BY last_export_at DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var acc Account
		err := rows.Scan(&acc.ID, &acc.AccountKey, &acc.DisplayName, &acc.EbayUserID, &acc.EbayUsername,
			&acc.Environment, &acc.MarketplaceID, &acc.LastExportAt, &acc.CreatedAt, &acc.UpdatedAt)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

// GetAccountByKey retrieves an account by its unique key
func (db *DB) GetAccountByKey(accountKey string) (*Account, error) {
	var acc Account
	err := db.QueryRow(`
		SELECT id, account_key, display_name, COALESCE(ebay_user_id, ''), COALESCE(ebay_username, ''),
		       environment, marketplace_id, last_export_at, created_at, updated_at
		FROM accounts
		WHERE account_key = ?
	`, accountKey).Scan(&acc.ID, &acc.AccountKey, &acc.DisplayName, &acc.EbayUserID, &acc.EbayUsername,
		&acc.Environment, &acc.MarketplaceID, &acc.LastExportAt, &acc.CreatedAt, &acc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// CreateSyncHistory creates a new sync history record
func (db *DB) CreateSyncHistory(sh *SyncHistory) error {
	result, err := db.Exec(`
		INSERT INTO sync_history (account_id, sync_type, status, items_synced, error_message, started_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, sh.AccountID, sh.SyncType, sh.Status, sh.ItemsSynced, sh.ErrorMessage, sh.StartedAt)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	sh.ID = id
	return nil
}

// UpdateSyncHistory updates a sync history record
func (db *DB) UpdateSyncHistory(sh *SyncHistory) error {
	_, err := db.Exec(`
		UPDATE sync_history
		SET status = ?, items_synced = ?, error_message = ?, completed_at = ?
		WHERE id = ?
	`, sh.Status, sh.ItemsSynced, sh.ErrorMessage, sh.CompletedAt, sh.ID)
	return err
}

// GetSyncHistory returns sync history for an account
func (db *DB) GetSyncHistory(accountID int64, limit int) ([]SyncHistory, error) {
	rows, err := db.Query(`
		SELECT id, account_id, sync_type, status, items_synced, error_message, started_at, completed_at
		FROM sync_history
		WHERE account_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []SyncHistory
	for rows.Next() {
		var sh SyncHistory
		err := rows.Scan(&sh.ID, &sh.AccountID, &sh.SyncType, &sh.Status,
			&sh.ItemsSynced, &sh.ErrorMessage, &sh.StartedAt, &sh.CompletedAt)
		if err != nil {
			return nil, err
		}
		history = append(history, sh)
	}
	return history, rows.Err()
}

// BrandCOOMapping represents a brand to country of origin mapping
type BrandCOOMapping struct {
	ID         int64     `json:"id"`
	BrandName  string    `json:"brandName"`
	PrimaryCOO string    `json:"primaryCoo"`
	Notes      string    `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TariffRate represents a tariff rate by country
type TariffRate struct {
	ID            int64     `json:"id"`
	CountryName   string    `json:"countryName"`
	TariffRate    float64   `json:"tariffRate"`
	Notes         string    `json:"notes,omitempty"`
	EffectiveDate string    `json:"effectiveDate,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Setting represents an application setting (key-value pair)
type Setting struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description,omitempty"`
	DataType    string    `json:"dataType"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// GetAllSettings returns all application settings
func (db *DB) GetAllSettings() ([]Setting, error) {
	rows, err := db.Query(`
		SELECT id, key, value, COALESCE(description, ''), data_type, created_at, updated_at
		FROM settings
		ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		var s Setting
		err := rows.Scan(&s.ID, &s.Key, &s.Value, &s.Description, &s.DataType, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

// GetSetting returns a single setting by key
func (db *DB) GetSetting(key string) (*Setting, error) {
	var s Setting
	err := db.QueryRow(`
		SELECT id, key, value, COALESCE(description, ''), data_type, created_at, updated_at
		FROM settings
		WHERE key = ?
	`, key).Scan(&s.ID, &s.Key, &s.Value, &s.Description, &s.DataType, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // Setting not found
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateSetting updates the value of an existing setting
func (db *DB) UpdateSetting(key, value string) error {
	_, err := db.Exec(`
		UPDATE settings
		SET value = ?, updated_at = CURRENT_TIMESTAMP
		WHERE key = ?
	`, value, key)
	return err
}

// GetAllBrandCOOMappings returns all brand-COO mappings
func (db *DB) GetAllBrandCOOMappings() ([]BrandCOOMapping, error) {
	rows, err := db.Query(`
		SELECT id, brand_name, primary_coo, COALESCE(notes, ''), created_at, updated_at
		FROM brand_coo_mappings
		ORDER BY brand_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []BrandCOOMapping
	for rows.Next() {
		var m BrandCOOMapping
		err := rows.Scan(&m.ID, &m.BrandName, &m.PrimaryCOO, &m.Notes, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// GetBrandCOO returns the COO for a specific brand
func (db *DB) GetBrandCOO(brandName string) (string, error) {
	var coo string
	err := db.QueryRow(`
		SELECT primary_coo
		FROM brand_coo_mappings
		WHERE brand_name = ?
	`, brandName).Scan(&coo)
	if err == sql.ErrNoRows {
		return "", nil // Brand not found, return empty string
	}
	return coo, err
}

// CreateBrandCOOMapping creates a new brand-COO mapping
func (db *DB) CreateBrandCOOMapping(brandName, primaryCOO, notes string) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO brand_coo_mappings (brand_name, primary_coo, notes)
		VALUES (?, ?, ?)
	`, brandName, primaryCOO, notes)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateBrandCOOMapping updates an existing brand-COO mapping
func (db *DB) UpdateBrandCOOMapping(id int64, brandName, primaryCOO, notes string) error {
	_, err := db.Exec(`
		UPDATE brand_coo_mappings
		SET brand_name = ?, primary_coo = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, brandName, primaryCOO, notes, id)
	return err
}

// DeleteBrandCOOMapping deletes a brand-COO mapping
func (db *DB) DeleteBrandCOOMapping(id int64) error {
	_, err := db.Exec("DELETE FROM brand_coo_mappings WHERE id = ?", id)
	return err
}

// GetAllTariffRates returns all tariff rates
func (db *DB) GetAllTariffRates() ([]TariffRate, error) {
	rows, err := db.Query(`
		SELECT id, country_name, tariff_rate, COALESCE(notes, ''), COALESCE(effective_date, ''), created_at, updated_at
		FROM tariff_rates
		ORDER BY country_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []TariffRate
	for rows.Next() {
		var r TariffRate
		err := rows.Scan(&r.ID, &r.CountryName, &r.TariffRate, &r.Notes, &r.EffectiveDate, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		rates = append(rates, r)
	}
	return rates, rows.Err()
}

// GetTariffRate returns the tariff rate for a specific country
func (db *DB) GetTariffRate(countryName string) (float64, error) {
	var rate float64
	err := db.QueryRow(`
		SELECT tariff_rate
		FROM tariff_rates
		WHERE country_name = ?
	`, countryName).Scan(&rate)
	if err == sql.ErrNoRows {
		return 0, nil // Country not found, return 0%
	}
	return rate, err
}

// DeletionNotification represents a marketplace account deletion notification from eBay
type DeletionNotification struct {
	ID             int64     `json:"id"`
	NotificationID string    `json:"notificationId"`
	Username       string    `json:"username"`
	UserID         string    `json:"userId,omitempty"`
	EiasToken      string    `json:"eiasToken,omitempty"`
	EventDate      time.Time `json:"eventDate"`
	ReceivedAt     time.Time `json:"receivedAt"`
	Processed      bool      `json:"processed"`
	ProcessedAt    *time.Time `json:"processedAt,omitempty"`
	RawPayload     string    `json:"rawPayload"`
}

// CreateDeletionNotification stores a new deletion notification
func (db *DB) CreateDeletionNotification(dn *DeletionNotification) error {
	_, err := db.Exec(`
		INSERT INTO deletion_notifications
		(notification_id, username, user_id, eias_token, event_date, raw_payload)
		VALUES (?, ?, ?, ?, ?, ?)
	`, dn.NotificationID, dn.Username, dn.UserID, dn.EiasToken, dn.EventDate, dn.RawPayload)
	return err
}

// GetDeletionNotifications returns all deletion notifications
func (db *DB) GetDeletionNotifications(limit int) ([]DeletionNotification, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query(`
		SELECT id, notification_id, username, user_id, eias_token,
		       event_date, received_at, processed, processed_at, raw_payload
		FROM deletion_notifications
		ORDER BY received_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []DeletionNotification
	for rows.Next() {
		var dn DeletionNotification
		err := rows.Scan(&dn.ID, &dn.NotificationID, &dn.Username, &dn.UserID,
			&dn.EiasToken, &dn.EventDate, &dn.ReceivedAt, &dn.Processed,
			&dn.ProcessedAt, &dn.RawPayload)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, dn)
	}
	return notifications, rows.Err()
}

// MarkDeletionNotificationProcessed marks a notification as processed
func (db *DB) MarkDeletionNotificationProcessed(notificationID string) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE deletion_notifications
		SET processed = TRUE, processed_at = ?
		WHERE notification_id = ?
	`, now, notificationID)
	return err
}

// SeedInitialData seeds the database with initial brand-COO mappings and tariff rates
func (db *DB) SeedInitialData() error {
	// Check if already seeded
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM brand_coo_mappings").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // Already seeded
	}

	// Seed brand-COO mappings (from calculator/data.go)
	brandMappings := map[string]string{
		"Alice McCall": "China", "Arnhem": "India", "Bec + Bridge": "China",
		"Bronx and Banco": "China", "Camilla": "India", "Faithfull The Brand": "Indonesia",
		"Free People": "China", "Kookai": "China", "Lack of Color": "China",
		"Lele Sadoughi": "United States", "Love Bonfire": "China", "LoveShackFancy": "China",
		"Nine Lives Bazaar": "China", "Reebok x Maison": "Vietnam", "Sabbi": "Australia",
		"Selkie": "China", "Spell": "China", "Tree of Life": "India", "Wildfox": "China",
	}

	for brand, coo := range brandMappings {
		if _, err := db.CreateBrandCOOMapping(brand, coo, ""); err != nil {
			return fmt.Errorf("failed to seed brand %s: %w", brand, err)
		}
	}

	// Seed tariff rates (from calculator/data.go)
	tariffRates := map[string]float64{
		"China": 0.20, "India": 0.50, "Indonesia": 0.19, "Vietnam": 0.20,
		"Mexico": 0.25, "Australia": 0.10, "United States": 0.00,
	}

	for country, rate := range tariffRates {
		_, err := db.Exec(`
			INSERT INTO tariff_rates (country_name, tariff_rate, notes, effective_date)
			VALUES (?, ?, ?, ?)
		`, country, rate, "IEEPA Reciprocal Tariff", "2025-02-01")
		if err != nil {
			return fmt.Errorf("failed to seed tariff for %s: %w", country, err)
		}
	}

	return nil
}

// EnrichedItem represents cached enriched item data from GetItem API
type EnrichedItem struct {
	ItemID           string    `json:"itemId"`
	Brand            string    `json:"brand"`
	CountryOfOrigin  string    `json:"countryOfOrigin"`
	ShippingCost     string    `json:"shippingCost"`
	ShippingCurrency string    `json:"shippingCurrency"`
	EnrichedAt       time.Time `json:"enrichedAt"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// GetEnrichedItem retrieves cached enriched data for an item
// Returns nil if not found or expired (based on TTL)
func (db *DB) GetEnrichedItem(itemID string, ttlDays int) (*EnrichedItem, error) {
	var item EnrichedItem
	err := db.QueryRow(`
		SELECT item_id, COALESCE(brand, ''), COALESCE(country_of_origin, ''),
		       COALESCE(shipping_cost, ''), COALESCE(shipping_currency, ''),
		       enriched_at, created_at, updated_at
		FROM enriched_items
		WHERE item_id = ?
	`, itemID).Scan(&item.ItemID, &item.Brand, &item.CountryOfOrigin,
		&item.ShippingCost, &item.ShippingCurrency, &item.EnrichedAt,
		&item.CreatedAt, &item.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	// Check TTL - if expired, return nil
	if time.Since(item.EnrichedAt) > time.Duration(ttlDays)*24*time.Hour {
		return nil, nil // Expired
	}

	return &item, nil
}

// SaveEnrichedItem saves or updates enriched item data
func (db *DB) SaveEnrichedItem(item *EnrichedItem) error {
	_, err := db.Exec(`
		INSERT INTO enriched_items (item_id, brand, country_of_origin, shipping_cost, shipping_currency, enriched_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(item_id) DO UPDATE SET
			brand = excluded.brand,
			country_of_origin = excluded.country_of_origin,
			shipping_cost = excluded.shipping_cost,
			shipping_currency = excluded.shipping_currency,
			enriched_at = excluded.enriched_at,
			updated_at = CURRENT_TIMESTAMP
	`, item.ItemID, item.Brand, item.CountryOfOrigin, item.ShippingCost, item.ShippingCurrency, item.EnrichedAt)
	return err
}

// GetEnrichedItemsBatch retrieves multiple enriched items at once
// Returns a map of itemID -> EnrichedItem for items that exist and are not expired
func (db *DB) GetEnrichedItemsBatch(itemIDs []string, ttlDays int) (map[string]*EnrichedItem, error) {
	result := make(map[string]*EnrichedItem)

	if len(itemIDs) == 0 {
		return result, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]interface{}, len(itemIDs))
	for i, id := range itemIDs {
		placeholders[i] = id
	}

	// Create the query with proper number of placeholders
	query := `
		SELECT item_id, COALESCE(brand, ''), COALESCE(country_of_origin, ''),
		       COALESCE(shipping_cost, ''), COALESCE(shipping_currency, ''),
		       enriched_at, created_at, updated_at
		FROM enriched_items
		WHERE item_id IN (?` + generatePlaceholders(len(itemIDs)-1) + `)`

	rows, err := db.Query(query, placeholders...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cutoffTime := time.Now().Add(-time.Duration(ttlDays) * 24 * time.Hour)

	for rows.Next() {
		var item EnrichedItem
		err := rows.Scan(&item.ItemID, &item.Brand, &item.CountryOfOrigin,
			&item.ShippingCost, &item.ShippingCurrency, &item.EnrichedAt,
			&item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Only include if not expired
		if item.EnrichedAt.After(cutoffTime) {
			result[item.ItemID] = &item
		}
	}

	return result, rows.Err()
}

// Helper function to generate SQL placeholders for batch queries
func generatePlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < count; i++ {
		result += ", ?"
	}
	return result
}

// ListingItem represents a fully enriched listing for the frontend
type ListingItem struct {
	ItemID           string   `json:"itemId"`
	OfferID          string   `json:"offerId"`
	Title            string   `json:"title"`
	Price            float64  `json:"price"`
	Currency         string   `json:"currency"`
	ImageURL         string   `json:"imageUrl"`
	Brand            string   `json:"brand"`
	CountryOfOrigin  string   `json:"countryOfOrigin"`
	ExpectedCOO      string   `json:"expectedCoo"`      // From brand mapping
	COOMatch         string   `json:"cooMatch"`         // "match", "mismatch", "missing"
	WeightBand       string   `json:"weightBand"`
	ShippingCost     float64  `json:"shippingCost"`
	CalculatedCost   float64  `json:"calculatedCost"`   // Server-calculated postage
	Diff             float64  `json:"diff"`             // ShippingCost - CalculatedCost
	DiffStatus       string   `json:"diffStatus"`       // "ok" (green) or "bad" (red)
	Images           []string `json:"images"`
}

// ListingsQuery represents query parameters for listing search
type ListingsQuery struct {
	Search    string
	SortBy    string // title, price, brand, coo, shipping, calculated, diff
	SortOrder string // asc, desc
	Page      int
	PageSize  int
}

// ListingsResult represents paginated listings response
type ListingsResult struct {
	Items      []ListingItem `json:"items"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	TotalPages int           `json:"totalPages"`
}

// GetListings retrieves enriched listings with sorting, filtering, and pagination
// All business logic (COO matching, postage calculation) happens server-side
func (db *DB) GetListings(query ListingsQuery) (*ListingsResult, error) {
	// Build the query with JOINs to get all data
	baseQuery := `
		SELECT
			e.item_id,
			e.item_id as offer_id,
			COALESCE(e.brand, '') as brand,
			COALESCE(e.country_of_origin, '') as country_of_origin,
			COALESCE(e.shipping_cost, '0') as shipping_cost,
			COALESCE(e.images, '[]') as images,
			COALESCE(bcm.primary_coo, 'China') as expected_coo,
			COALESCE(tr.tariff_rate, 0.20) as tariff_rate
		FROM enriched_items e
		LEFT JOIN brand_coo_mappings bcm ON LOWER(e.brand) = LOWER(bcm.brand_name)
		LEFT JOIN tariff_rates tr ON LOWER(COALESCE(e.country_of_origin, bcm.primary_coo, 'China')) = LOWER(tr.country_name)
		WHERE 1=1
	`

	var args []interface{}

	// Add search filter
	if query.Search != "" {
		baseQuery += " AND (LOWER(e.brand) LIKE ? OR LOWER(e.item_id) LIKE ?)"
		searchTerm := "%" + query.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM (" + baseQuery + ")"
	var total int
	err := db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count listings: %w", err)
	}

	// Add sorting
	orderBy := " ORDER BY "
	switch query.SortBy {
	case "brand":
		orderBy += "brand"
	case "coo":
		orderBy += "country_of_origin"
	case "shipping":
		orderBy += "CAST(shipping_cost AS REAL)"
	default:
		orderBy += "e.item_id"
	}
	if query.SortOrder == "desc" {
		orderBy += " DESC"
	} else {
		orderBy += " ASC"
	}
	baseQuery += orderBy

	// Add pagination
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.Page < 0 {
		query.Page = 0
	}
	offset := query.Page * query.PageSize
	baseQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", query.PageSize, offset)

	// Execute query
	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query listings: %w", err)
	}
	defer rows.Close()

	var items []ListingItem
	for rows.Next() {
		var item ListingItem
		var imagesJSON string
		var tariffRate float64
		var shippingCostStr string

		err := rows.Scan(
			&item.ItemID,
			&item.OfferID,
			&item.Brand,
			&item.CountryOfOrigin,
			&shippingCostStr,
			&imagesJSON,
			&item.ExpectedCOO,
			&tariffRate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan listing: %w", err)
		}

		// Parse shipping cost
		fmt.Sscanf(shippingCostStr, "%f", &item.ShippingCost)

		// Calculate COO match status
		if item.CountryOfOrigin == "" {
			item.COOMatch = "missing"
		} else if item.CountryOfOrigin == item.ExpectedCOO {
			item.COOMatch = "match"
		} else {
			item.COOMatch = "mismatch"
		}

		// Server-side postage calculation
		item.CalculatedCost = calculatePostage(item.Price, tariffRate)
		item.Diff = item.ShippingCost - item.CalculatedCost

		// 5% threshold for diff status
		threshold := item.CalculatedCost * 1.05
		if item.ShippingCost >= threshold {
			item.DiffStatus = "ok"
		} else {
			item.DiffStatus = "bad"
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	totalPages := (total + query.PageSize - 1) / query.PageSize

	return &ListingsResult{
		Items:      items,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

// Server-side postage calculation
// Formula: AusPost Shipping + Extra Cover + Tariff Duties + Zonos Fees
func calculatePostage(price, tariffRate float64) float64 {
	const (
		handlingFee       = 0.02
		zonosPercentage   = 0.10
		zonosFixedCost    = 1.69
		extraCoverBase    = 4.00
		extraCoverDiscount = 0.40
		extraCoverThreshold = 100.0
		savingsDiscount   = 0.175 // Band 3 default
		ausPostBase       = 60.00 // Medium weight band
	)

	// AusPost shipping with handling fee and savings discount
	ausPostShipping := ausPostBase * (1 + handlingFee) * (1 - savingsDiscount)

	// Extra cover for items over $100
	var extraCover float64
	if price > extraCoverThreshold {
		coverableAmount := price - extraCoverThreshold
		coverUnits := (coverableAmount + 99) / 100 // Ceiling division
		extraCover = coverUnits * extraCoverBase * (1 - extraCoverDiscount)
	}

	// Tariff duties
	tariffDuties := price * tariffRate

	// Zonos fees
	zonosFees := (tariffDuties * zonosPercentage) + zonosFixedCost

	return ausPostShipping + extraCover + tariffDuties + zonosFees
}
