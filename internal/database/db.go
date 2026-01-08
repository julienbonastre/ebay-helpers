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
	AccountKey    string     `json:"accountKey"`    // Unique key: "storename_env_marketplace"
	DisplayName   string     `json:"displayName"`   // Human readable: "La Troverie Production"
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
		SELECT id, account_key, display_name, environment, marketplace_id, last_export_at, created_at, updated_at
		FROM accounts
		WHERE account_key = ?
	`, accountKey).Scan(&acc.ID, &acc.AccountKey, &acc.DisplayName, &acc.Environment,
		&acc.MarketplaceID, &acc.LastExportAt, &acc.CreatedAt, &acc.UpdatedAt)

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
		SELECT id, account_key, display_name, environment, marketplace_id, last_export_at, created_at, updated_at
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
		err := rows.Scan(&acc.ID, &acc.AccountKey, &acc.DisplayName, &acc.Environment,
			&acc.MarketplaceID, &acc.LastExportAt, &acc.CreatedAt, &acc.UpdatedAt)
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
		SELECT id, account_key, display_name, environment, marketplace_id, last_export_at, created_at, updated_at
		FROM accounts
		WHERE account_key = ?
	`, accountKey).Scan(&acc.ID, &acc.AccountKey, &acc.DisplayName, &acc.Environment,
		&acc.MarketplaceID, &acc.LastExportAt, &acc.CreatedAt, &acc.UpdatedAt)
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
