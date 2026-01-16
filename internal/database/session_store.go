package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// DBSessionStore implements gorilla/sessions.Store using SQLite database
// Stores only session ID in cookie, actual session data in database
type DBSessionStore struct {
	db      *DB
	codecs  []securecookie.Codec
	options *sessions.Options
}

// NewDBSessionStore creates a new database-backed session store
func NewDBSessionStore(db *DB, keyPairs ...[]byte) *DBSessionStore {
	return &DBSessionStore{
		db:     db,
		codecs: securecookie.CodecsFromPairs(keyPairs...),
		options: &sessions.Options{
			Path:     "/",
			MaxAge:   86400 * 30, // 30 days
			HttpOnly: true,
			Secure:   false, // Set to true in production with HTTPS
			SameSite: http.SameSiteLaxMode,
		},
	}
}

// SetOptions sets the session options
func (s *DBSessionStore) SetOptions(options *sessions.Options) {
	s.options = options
}

// Get returns a session for the given name after adding it to the registry
func (s *DBSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New creates a new session
func (s *DBSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	session.IsNew = true

	// Try to load existing session from cookie
	cookie, err := r.Cookie(name)
	if err != nil {
		// No existing session
		return session, nil
	}

	// Decode session ID from cookie
	sessionID := ""
	err = securecookie.DecodeMulti(name, cookie.Value, &sessionID, s.codecs...)
	if err != nil {
		// Invalid cookie, return new session
		return session, nil
	}

	// Load session data from database
	data, err := s.loadFromDB(sessionID)
	if err != nil {
		// Session not found or expired, return new session
		return session, nil
	}

	// Unmarshal session values into a temporary map
	// JSON unmarshals to map[string]interface{}, but session.Values is map[interface{}]interface{}
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return session, nil
	}

	// Convert map[string]interface{} to map[interface{}]interface{}
	for k, v := range values {
		session.Values[k] = v
	}

	session.ID = sessionID
	session.IsNew = false
	return session, nil
}

// Save persists the session to the database
func (s *DBSessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	// Delete session if MaxAge is negative
	if session.Options.MaxAge < 0 {
		if session.ID != "" {
			if err := s.deleteFromDB(session.ID); err != nil {
				return err
			}
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	// Generate new session ID if needed
	if session.ID == "" {
		session.ID = s.generateSessionID()
	}

	// Convert map[interface{}]interface{} to map[string]interface{} for JSON marshaling
	// gorilla/sessions uses interface{} keys, but JSON requires string keys
	values := make(map[string]interface{})
	for k, v := range session.Values {
		if key, ok := k.(string); ok {
			values[key] = v
		}
	}

	// Marshal session values to JSON
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(session.Options.MaxAge) * time.Second)

	// Save to database
	if err := s.saveToDB(session.ID, data, expiresAt); err != nil {
		return err
	}

	// Encode session ID into cookie value
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
	if err != nil {
		return err
	}

	// Set cookie
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

// generateSessionID creates a random session identifier
func (s *DBSessionStore) generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)
}

// saveToDB stores session data in the database
func (s *DBSessionStore) saveToDB(sessionID string, data []byte, expiresAt time.Time) error {
	query := `
		INSERT INTO sessions (session_id, data, expires_at)
		VALUES (?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			data = excluded.data,
			expires_at = excluded.expires_at
	`
	_, err := s.db.DB.Exec(query, sessionID, string(data), expiresAt)
	return err
}

// loadFromDB retrieves session data from the database
func (s *DBSessionStore) loadFromDB(sessionID string) ([]byte, error) {
	query := `
		SELECT data FROM sessions
		WHERE session_id = ? AND expires_at > datetime('now')
	`
	var data string
	err := s.db.DB.QueryRow(query, sessionID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found or expired")
	}
	if err != nil {
		return nil, err
	}
	return []byte(data), nil
}

// deleteFromDB removes a session from the database
func (s *DBSessionStore) deleteFromDB(sessionID string) error {
	query := `DELETE FROM sessions WHERE session_id = ?`
	_, err := s.db.DB.Exec(query, sessionID)
	return err
}

// CleanupExpiredSessions removes expired sessions from the database
// Should be called periodically (e.g., daily cron job)
func (s *DBSessionStore) CleanupExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expires_at <= datetime('now')`
	_, err := s.db.DB.Exec(query)
	return err
}
