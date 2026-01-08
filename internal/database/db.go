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

// Account represents an eBay account profile
type Account struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Environment   string     `json:"environment"` // "production" or "sandbox"
	MarketplaceID string     `json:"marketplaceId"`
	ClientID      string     `json:"clientId,omitempty"`
	ClientSecret  string     `json:"clientSecret,omitempty"`
	RedirectURI   string     `json:"redirectUri,omitempty"`

	// OAuth tokens (omitted from JSON for security)
	AccessToken   string     `json:"-"`
	RefreshToken  string     `json:"-"`
	TokenType     string     `json:"-"`
	TokenExpiry   *time.Time `json:"-"`

	IsActive      bool       `json:"isActive"`
	IsConnected   bool       `json:"isConnected"` // Has valid tokens
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

// CreateAccount creates a new account profile
func (db *DB) CreateAccount(acc *Account) error {
	result, err := db.Exec(`
		INSERT INTO accounts (name, environment, marketplace_id, client_id, client_secret, redirect_uri, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, acc.Name, acc.Environment, acc.MarketplaceID, acc.ClientID, acc.ClientSecret, acc.RedirectURI, acc.IsActive)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	acc.ID = id
	return nil
}

// UpdateAccountCredentials updates just the OAuth credentials for an account
func (db *DB) UpdateAccountCredentials(accountID int64, clientID, clientSecret string) error {
	_, err := db.Exec(`
		UPDATE accounts
		SET client_id = ?, client_secret = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, clientID, clientSecret, accountID)
	return err
}

// SaveOAuthToken saves or updates OAuth tokens for an account
func (db *DB) SaveOAuthToken(accountID int64, accessToken, refreshToken, tokenType string, expiry time.Time) error {
	_, err := db.Exec(`
		UPDATE accounts
		SET access_token = ?, refresh_token = ?, token_type = ?, token_expiry = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, accessToken, refreshToken, tokenType, expiry, accountID)
	return err
}

// HasValidToken checks if an account has a non-expired access token
func (db *DB) HasValidToken(accountID int64) (bool, error) {
	var tokenExpiry sql.NullTime
	err := db.QueryRow(`
		SELECT token_expiry
		FROM accounts
		WHERE id = ? AND access_token IS NOT NULL AND access_token != ''
	`, accountID).Scan(&tokenExpiry)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !tokenExpiry.Valid {
		return false, nil
	}

	// Token is valid if it expires in the future (with 5 min buffer)
	return tokenExpiry.Time.After(time.Now().Add(5 * time.Minute)), nil
}

// GetAccounts returns all account profiles
func (db *DB) GetAccounts() ([]Account, error) {
	rows, err := db.Query(`
		SELECT id, name, environment, marketplace_id, client_id, client_secret, redirect_uri, is_active, created_at, updated_at
		FROM accounts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var acc Account
		err := rows.Scan(&acc.ID, &acc.Name, &acc.Environment, &acc.MarketplaceID,
			&acc.ClientID, &acc.ClientSecret, &acc.RedirectURI, &acc.IsActive,
			&acc.CreatedAt, &acc.UpdatedAt)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

// GetActiveAccount returns the currently active account
func (db *DB) GetActiveAccount() (*Account, error) {
	var acc Account
	err := db.QueryRow(`
		SELECT id, name, environment, marketplace_id, client_id, client_secret, redirect_uri, is_active, created_at, updated_at
		FROM accounts
		WHERE is_active = 1
		LIMIT 1
	`).Scan(&acc.ID, &acc.Name, &acc.Environment, &acc.MarketplaceID,
		&acc.ClientID, &acc.ClientSecret, &acc.RedirectURI, &acc.IsActive,
		&acc.CreatedAt, &acc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// SetActiveAccount sets an account as active (deactivates others)
func (db *DB) SetActiveAccount(accountID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Deactivate all accounts
	if _, err := tx.Exec("UPDATE accounts SET is_active = 0"); err != nil {
		return err
	}

	// Activate the specified account
	if _, err := tx.Exec("UPDATE accounts SET is_active = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", accountID); err != nil {
		return err
	}

	return tx.Commit()
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
