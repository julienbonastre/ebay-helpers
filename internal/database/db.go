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
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"` // "production" or "sandbox"
	MarketplaceID string    `json:"marketplaceId"`
	ClientID      string    `json:"clientId,omitempty"`
	ClientSecret  string    `json:"clientSecret,omitempty"`
	RedirectURI   string    `json:"redirectUri,omitempty"`
	IsActive      bool      `json:"isActive"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
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
