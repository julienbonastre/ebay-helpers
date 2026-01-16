package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienbonastre/ebay-helpers/internal/calculator"
	"github.com/julienbonastre/ebay-helpers/internal/database"
	"github.com/julienbonastre/ebay-helpers/internal/ebay"
	syncpkg "github.com/julienbonastre/ebay-helpers/internal/sync"
	"golang.org/x/oauth2"
)

// EnrichedItemData holds enriched item details from GetItem API
// Now includes server-calculated postage to keep business logic on backend
type EnrichedItemData struct {
	ItemID           string    `json:"itemId"`
	Brand            string    `json:"brand"`
	CountryOfOrigin  string    `json:"countryOfOrigin"`
	ExpectedCOO      string    `json:"expectedCoo"`      // From brand mapping
	COOStatus        string    `json:"cooStatus"`        // "match", "mismatch", "missing"
	ShippingCost     string    `json:"shippingCost"`
	ShippingCurrency string    `json:"shippingCurrency"`
	CalculatedCost   float64   `json:"calculatedCost"`   // Server-calculated postage
	Diff             float64   `json:"diff"`             // ShippingCost - CalculatedCost
	DiffStatus       string    `json:"diffStatus"`       // "ok" (green) or "bad" (red)
	Images           []string  `json:"images"`
	EnrichedAt       time.Time `json:"enrichedAt"`
}

// Handler holds dependencies for HTTP handlers
type Handler struct {
	db                *database.DB
	ebayConfig        ebay.Config                // eBay configuration (no shared client)
	sessionStore      *database.DBSessionStore   // Session store for per-user tokens
	currentAccount    *database.Account          // Current instance's account (can be nil until OAuth)
	syncService       *syncpkg.Service
	mu                sync.RWMutex
	oauthState        string
	verificationToken string                     // eBay verification token for account deletion notifications
	endpoint          string                     // Public endpoint URL for this server
	environment       string                     // "production" or "sandbox"
	marketplaceID     string                     // Default marketplace ID

	// Item enrichment cache and background worker
	enrichmentCache   map[string]*EnrichedItemData // ItemID -> EnrichedItemData
	enrichmentMutex   sync.RWMutex                 // Protects enrichmentCache
	enrichmentQueue   chan string                  // Queue of ItemIDs to enrich

	// Listings cache - avoids re-fetching from eBay on every page load
	listingsCache     []map[string]interface{}     // Cached offer listings
	listingsCacheTime time.Time                    // When cache was last updated
	listingsMutex     sync.RWMutex                 // Protects listingsCache
}

// NewHandler creates a new handler
func NewHandler(db *database.DB, config ebay.Config, sessionStore *database.DBSessionStore, verificationToken, endpoint, environment, marketplaceID string) *Handler {
	h := &Handler{
		db:                db,
		ebayConfig:        config,
		sessionStore:      sessionStore,
		currentAccount:    nil, // Will be set after OAuth
		syncService:       syncpkg.NewService(db),
		verificationToken: verificationToken,
		endpoint:          endpoint,
		environment:       environment,
		marketplaceID:     marketplaceID,
		enrichmentCache:   make(map[string]*EnrichedItemData),
		enrichmentQueue:   make(chan string, 1000), // Buffer up to 1000 items
	}

	// TODO: Background enrichment worker disabled for session-based auth
	// The enrichment worker ran in a background goroutine without HTTP request context,
	// which means it couldn't access session-based OAuth tokens.
	// To re-enable, refactor to either:
	// 1. Make enrichment on-demand per request, or
	// 2. Store a reference to the current user's token (complex with multi-user sessions)
	// go h.enrichmentWorker()

	return h
}

// Session constants
const (
	sessionName = "ebay-helper-session"
	tokenKey    = "oauth_token"
)

// getEbayClient creates a client for this request using session token
func (h *Handler) getEbayClient(r *http.Request) (*ebay.Client, error) {
	session, err := h.sessionStore.Get(r, sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}


	client := ebay.NewClient(h.ebayConfig)

	// Load token from session if it exists
	// Note: token may be []byte (in-memory) or string (from database JSON)
	if tokenData, ok := session.Values[tokenKey].([]byte); ok {
		var token oauth2.Token
		if err := json.Unmarshal(tokenData, &token); err == nil {
			client.SetToken(&token)
		} else {
		}
	} else if tokenStr, ok := session.Values[tokenKey].(string); ok {
		// When loaded from database, []byte becomes base64-encoded string after JSON round-trip
		// Need to base64-decode first, then unmarshal
		tokenBytes, err := base64.StdEncoding.DecodeString(tokenStr)
		if err != nil {
		} else {
			var token oauth2.Token
			if err := json.Unmarshal(tokenBytes, &token); err == nil {
				client.SetToken(&token)
			} else {
			}
		}
	}

	return client, nil
}

// saveTokenToSession stores the OAuth token in the session
func (h *Handler) saveTokenToSession(w http.ResponseWriter, r *http.Request, token *oauth2.Token) error {
	session, err := h.sessionStore.Get(r, sessionName)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	tokenData, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	session.Values[tokenKey] = tokenData
	return session.Save(r, w)
}

// clearSession removes all session data
func (h *Handler) clearSession(w http.ResponseWriter, r *http.Request) error {
	session, err := h.sessionStore.Get(r, sessionName)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

// TODO: enrichmentWorker disabled for session-based auth
// The enrichmentWorker ran in a background goroutine without HTTP request context,
// which means it couldn't access session-based OAuth tokens.
// To re-enable, refactor to either:
// 1. Make enrichment on-demand per request, or
// 2. Store a reference to the current user's token (complex with multi-user sessions)
/*
func (h *Handler) enrichmentWorker() {
	const numWorkers = 25 // Process 25 items concurrently
	log.Printf("[ENRICHMENT] Background worker started with %d concurrent workers", numWorkers)

	// Create worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for itemID := range h.enrichmentQueue {
				// Check if already enriched
				h.enrichmentMutex.RLock()
				_, exists := h.enrichmentCache[itemID]
				h.enrichmentMutex.RUnlock()

				if exists {
					continue // Already enriched
				}

				// Fetch item details using GetItem
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				// NOTE: Can't use h.ebayClient anymore with session-based auth
				// brand, shippingCost, shippingCurrency, coo, images, err := h.ebayClient.GetItem(ctx, itemID)
				cancel()

				// Store empty entry to avoid retrying failed items
				h.enrichmentMutex.Lock()
				h.enrichmentCache[itemID] = &EnrichedItemData{
					ItemID:     itemID,
					EnrichedAt: time.Now(),
				}
				h.enrichmentMutex.Unlock()
			}
		}(i)
	}

	// Wait for all workers to finish (this won't happen until channel is closed)
	wg.Wait()
	log.Printf("[ENRICHMENT] All workers stopped")
}

func (h *Handler) queueItemsForEnrichment(itemIDs []string) {
	for _, itemID := range itemIDs {
		select {
		case h.enrichmentQueue <- itemID:
			// Queued successfully
		default:
			// Queue is full, skip this item
			log.Printf("[ENRICHMENT] Queue full, skipping item %s", itemID)
		}
	}
}
*/

// JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// Error response helper
func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// HealthCheck returns API health status
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	client, err := h.getEbayClient(r)
	authenticated := false
	if err == nil {
		authenticated = client.IsAuthenticated()
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"authenticated": authenticated,
		"configured":    h.ebayConfig.ClientID != "",
		"hasAccount":    h.currentAccount != nil,
	})
}

// GetCurrentAccount returns the current instance's account info
func (h *Handler) GetCurrentAccount(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	account := h.currentAccount
	h.mu.RUnlock()


	// If no account in memory but user has valid session, hydrate from eBay
	if account == nil {
		client, err := h.getEbayClient(r)
		if err == nil && client.IsAuthenticated() {

			// Fetch user info from eBay
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			user, err := client.GetUser(ctx)
			cancel()

			if err == nil && user != nil {
				// Create/update account in database
				accountKey := fmt.Sprintf("%s_%s", user.UserID, h.environment)
				dbAccount, err := h.db.GetOrCreateAccountFromEbay(accountKey, user.Username, h.environment, h.marketplaceID)
				if err == nil {
					h.mu.Lock()
					h.currentAccount = dbAccount
					account = dbAccount
					h.mu.Unlock()
				} else {
				}
			} else {
			}
		}
	}

	if account == nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"configured": false,
			"message":    "Not connected to an eBay account. Authenticate to continue.",
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"configured": true,
		"account":    account,
	})
}

// GetAccounts returns all accounts that have data in the database
func (h *Handler) GetAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.db.GetAccounts()
	if err != nil {
		log.Printf("GetAccounts error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"accounts": accounts,
		"total":    len(accounts),
	})
}

// GetAuthURL returns the OAuth authorization URL
func (h *Handler) GetAuthURL(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.oauthState = generateState()
	state := h.oauthState
	h.mu.Unlock()

	client := ebay.NewClient(h.ebayConfig)
	url := client.GetAuthURL(state)
	jsonResponse(w, http.StatusOK, map[string]string{"url": url})
}

// OAuthCallback handles the OAuth callback
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	log.Printf("OAuth callback received - code: %v, state: %s, error: %s", code != "", state, errParam)

	if errParam != "" {
		log.Printf("OAuth error from eBay: %s - %s", errParam, errDesc)
		http.Error(w, "eBay OAuth error: "+errDesc, http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	expectedState := h.oauthState
	h.mu.RUnlock()

	log.Printf("State check - received: %s, expected: %s", state, expectedState)

	if state != expectedState {
		log.Printf("State mismatch!")
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	if code == "" {
		log.Printf("Missing authorization code")
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	log.Printf("Exchanging code for token...")
	client := ebay.NewClient(h.ebayConfig)
	if err := client.ExchangeCode(r.Context(), code); err != nil {
		log.Printf("OAuth exchange error: %v", err)
		http.Error(w, "Failed to authenticate: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the token from the client and save it to session
	token := client.GetToken()
	if token == nil {
		log.Printf("ERROR: Token is nil after exchange")
		http.Error(w, "Failed to obtain token", http.StatusInternalServerError)
		return
	}

	// Save token to session
	if err := h.saveTokenToSession(w, r, token); err != nil {
		log.Printf("Failed to save token to session: %v", err)
		http.Error(w, "Failed to save authentication", http.StatusInternalServerError)
		return
	}

	log.Printf("OAuth success! Token obtained and saved to session.")

	// Fetch eBay username using Commerce Identity API with retry logic
	// No useless fallbacks - if this fails, we show a proper error to the user
	var username, userID string
	var userErr error

	// Retry logic: 3 attempts with increasing timeout
	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		timeout := time.Duration(5+attempt*5) * time.Second // 10s, 15s, 20s
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		user, err := client.GetUser(ctx)
		cancel()

		if err == nil && user != nil {
			username = user.Username
			userID = user.UserID
			log.Printf("SUCCESS: Authenticated as eBay user: %s (ID: %s)", username, userID)
			userErr = nil
			break
		}

		userErr = err
		log.Printf("WARNING: Attempt %d failed: %v", attempt, err)

		if attempt < maxAttempts {
			backoff := time.Duration(attempt) * time.Second
			time.Sleep(backoff)
		}
	}

	if userErr != nil {
		log.Printf("ERROR: Failed to fetch eBay user info after %d attempts: %v", maxAttempts, userErr)
		http.Error(w, "Unable to connect to eBay to verify your account. Please try again later.", http.StatusServiceUnavailable)
		return
	}

	// Use a unique identifier based on the actual eBay user ID
	accountKey := fmt.Sprintf("%s_%s", userID, h.environment)

	// Create or update account with real eBay username
	account, err := h.db.GetOrCreateAccountFromEbay(accountKey, username, h.environment, h.marketplaceID)
	if err != nil {
		log.Printf("ERROR: Failed to create/update account: %v", err)
		http.Error(w, "Unable to create account. Please try again.", http.StatusInternalServerError)
		return
	}

	h.mu.Lock()
	h.currentAccount = account
	h.mu.Unlock()
	log.Printf("SUCCESS: Account created/updated: %s (AccountKey: %s)", account.DisplayName, account.AccountKey)

	// Redirect to the main app
	http.Redirect(w, r, "/?auth=success", http.StatusFound)
}

// GetAuthStatus returns current auth status
func (h *Handler) GetAuthStatus(w http.ResponseWriter, r *http.Request) {
	client, err := h.getEbayClient(r)
	authenticated := false
	if err == nil {
		authenticated = client.IsAuthenticated()
	} else {
	}

	configured := h.ebayConfig.ClientID != ""

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"authenticated": authenticated,
		"configured":    configured,
	})
}

// Logout clears the session and logs the user out
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.clearSession(w, r); err != nil {
		log.Printf("Failed to clear session: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to logout")
		return
	}

	// Also clear currentAccount on logout
	h.mu.Lock()
	h.currentAccount = nil
	h.mu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GetInventoryItems returns paginated inventory items
func (h *Handler) GetInventoryItems(w http.ResponseWriter, r *http.Request) {
	client, err := h.getEbayClient(r)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Session error")
		return
	}

	if !client.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}

	items, err := client.GetInventoryItems(r.Context(), limit, offset)
	if err != nil {
		log.Printf("GetInventoryItems error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, items)
}

// GetOffers returns paginated offers
// This endpoint uses the Trading API to fetch traditional eBay listings
func (h *Handler) GetOffers(w http.ResponseWriter, r *http.Request) {
	client, err := h.getEbayClient(r)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Session error")
		return
	}

	if !client.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	forceRefresh := r.URL.Query().Get("force") == "true"

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Check if we have cached listings and not forcing refresh
	h.listingsMutex.RLock()
	hasCachedListings := len(h.listingsCache) > 0
	cacheAge := time.Since(h.listingsCacheTime)
	h.listingsMutex.RUnlock()

	// Cache TTL: 8 hours (only Refresh button or server restart triggers re-fetch)
	const cacheTTL = 8 * time.Hour

	// Use cache if available, not forcing, and cache is within TTL
	if hasCachedListings && !forceRefresh && cacheAge < cacheTTL {
		log.Printf("[CACHE] Returning cached listings (age: %v, total: %d)", cacheAge.Round(time.Second), len(h.listingsCache))

		h.listingsMutex.RLock()
		total := len(h.listingsCache)

		// Paginate from cache
		end := offset + limit
		if end > total {
			end = total
		}
		var offers []map[string]interface{}
		if offset < total {
			offers = h.listingsCache[offset:end]
		}
		h.listingsMutex.RUnlock()

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"offers": offers,
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"cached": true,
		})
		return
	}

	// Need to fetch from eBay - fetch ALL listings CONCURRENTLY and cache them
	log.Printf("[CACHE] Fetching all listings from eBay CONCURRENTLY (force=%v, cacheAge=%v)", forceRefresh, cacheAge.Round(time.Second))

	startTime := time.Now()
	pageSize := 100 // Max allowed by Trading API

	// First, fetch page 1 to get total count
	log.Printf("[CACHE] Fetching page 1 to get total count...")
	firstPageItems, totalItems, err := client.GetMyeBaySelling(r.Context(), 1, pageSize)
	if err != nil {
		log.Printf("GetMyeBaySelling error: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to fetch listings: "+err.Error())
		return
	}

	totalPages := (totalItems + pageSize - 1) / pageSize
	log.Printf("[CACHE] Total items: %d, pages: %d", totalItems, totalPages)

	// Convert first page items
	convertItems := func(items []ebay.TradingItem) []map[string]interface{} {
		offers := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			offer := map[string]interface{}{
				"offerId": item.ItemID,
				"sku":     item.SKU,
				"title":   item.Title,
				"pricingSummary": map[string]interface{}{
					"price": map[string]interface{}{
						"value":    item.Price,
						"currency": item.Currency,
					},
				},
			}
			if item.ImageURL != "" {
				offer["image"] = map[string]interface{}{
					"imageUrl": item.ImageURL,
				}
			}
			if item.Brand != "" {
				offer["brand"] = item.Brand
			}
			if item.ShippingCost != "" {
				offer["shippingCost"] = map[string]interface{}{
					"value":    item.ShippingCost,
					"currency": item.ShippingCurrency,
				}
			}
			offers = append(offers, offer)
		}
		return offers
	}

	// Start with first page results
	allOffers := convertItems(firstPageItems)

	// If more pages, fetch them concurrently
	if totalPages > 1 {
		const maxWorkers = 5 // Concurrent requests to eBay (be nice, don't DDoS them!)

		type pageResult struct {
			pageNum int
			items   []ebay.TradingItem
			err     error
		}

		// Channel for page numbers to fetch
		pageChan := make(chan int, totalPages-1)
		// Channel for results
		resultChan := make(chan pageResult, totalPages-1)

		// Start worker goroutines
		var wg sync.WaitGroup
		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for pageNum := range pageChan {
					log.Printf("[CACHE-WORKER-%d] Fetching page %d...", workerID, pageNum)
					items, _, err := client.GetMyeBaySelling(r.Context(), pageNum, pageSize)
					resultChan <- pageResult{pageNum: pageNum, items: items, err: err}
				}
			}(i)
		}

		// Queue remaining pages (2 to totalPages)
		for p := 2; p <= totalPages; p++ {
			pageChan <- p
		}
		close(pageChan)

		// Wait for all workers to finish, then close results channel
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect results into a map (to preserve order)
		pageResults := make(map[int][]map[string]interface{})
		for result := range resultChan {
			if result.err != nil {
				log.Printf("[CACHE-ERROR] Page %d failed: %v", result.pageNum, result.err)
				continue // Skip failed pages rather than failing entirely
			}
			log.Printf("[CACHE] Page %d: got %d items", result.pageNum, len(result.items))
			pageResults[result.pageNum] = convertItems(result.items)
		}

		// Append results in order (page 2, 3, 4, ...)
		for p := 2; p <= totalPages; p++ {
			if offers, ok := pageResults[p]; ok {
				allOffers = append(allOffers, offers...)
			}
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("[CACHE] Fetched %d listings in %v (concurrent mode)", len(allOffers), elapsed.Round(time.Millisecond))

	// Update cache
	h.listingsMutex.Lock()
	h.listingsCache = allOffers
	h.listingsCacheTime = time.Now()
	h.listingsMutex.Unlock()

	log.Printf("[CACHE] Cached %d listings", len(allOffers))

	// Return paginated results
	total := len(allOffers)
	end := offset + limit
	if end > total {
		end = total
	}
	var offers []map[string]interface{}
	if offset < total {
		offers = allOffers[offset:end]
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"offers": offers,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"cached": false,
	})
}

// GetEnrichedData returns enriched item data, fetching on-demand using session-based OAuth
// This implements request-based enrichment with parallel fetching for better performance
func (h *Handler) GetEnrichedData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	// Parse itemIds from query parameters
	// Frontend sends: ?itemIds=id1,id2,id3
	itemIDsParam := r.URL.Query().Get("itemIds")
	if itemIDsParam == "" {
		errorResponse(w, http.StatusBadRequest, "No itemIds provided")
		return
	}

	// Split comma-separated item IDs
	var itemIDs []string
	for _, id := range strings.Split(itemIDsParam, ",") {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			itemIDs = append(itemIDs, trimmed)
		}
	}

	if len(itemIDs) == 0 {
		errorResponse(w, http.StatusBadRequest, "No valid itemIds provided")
		return
	}

	// Get eBay client using session-based auth (same as listings)
	client, err := h.getEbayClient(r)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Session error")
		return
	}

	if !client.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	// Prepare result map with mutex for concurrent writes
	result := make(map[string]EnrichedItemData)
	var resultMutex sync.Mutex

	// Separate items into cached and to-fetch
	var toFetch []string
	for _, itemID := range itemIDs {
		h.enrichmentMutex.RLock()
		cachedData, exists := h.enrichmentCache[itemID]
		h.enrichmentMutex.RUnlock()

		if exists && cachedData != nil {
			resultMutex.Lock()
			result[itemID] = *cachedData
			resultMutex.Unlock()
			log.Printf("[ENRICHMENT] Using cached data for item %s", itemID)
		} else {
			toFetch = append(toFetch, itemID)
		}
	}

	// Fetch uncached items in parallel (limit concurrency to 30)
	// eBay Trading API rate limits are typically 5000 calls/day for production
	// Each item = 1-2 API calls (Trading API + potential Browse API fallback)
	if len(toFetch) > 0 {
		const maxConcurrent = 30
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		log.Printf("[ENRICHMENT] Fetching %d items in parallel (max %d concurrent)", len(toFetch), maxConcurrent)

		for _, itemID := range toFetch {
			wg.Add(1)
			sem <- struct{}{} // Acquire semaphore

			go func(id string) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore

				// Retry with exponential backoff
				var enrichedData *EnrichedItemData
				maxRetries := 3
				for attempt := 1; attempt <= maxRetries; attempt++ {
					log.Printf("[ENRICHMENT] Fetching item %s (attempt %d/%d)", id, attempt, maxRetries)
					ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
					brand, shippingCost, shippingCurrency, coo, images, err := client.GetItem(ctx, id)
					cancel()

					if err == nil {
						enrichedData = &EnrichedItemData{
							ItemID:           id,
							Brand:            brand,
							CountryOfOrigin:  coo,
							ShippingCost:     shippingCost,
							ShippingCurrency: shippingCurrency,
							Images:           images,
							EnrichedAt:       time.Now(),
						}
						log.Printf("[ENRICHMENT] Successfully enriched item %s (Brand: %s, COO: %s, Images: %d)",
							id, brand, coo, len(images))
						break
					}

					// Check for rate limiting (HTTP 429) or server errors (5xx)
					errMsg := err.Error()
					isRetryable := strings.Contains(errMsg, "429") ||
						strings.Contains(errMsg, "500") ||
						strings.Contains(errMsg, "502") ||
						strings.Contains(errMsg, "503") ||
						strings.Contains(errMsg, "timeout")

					if !isRetryable || attempt == maxRetries {
						log.Printf("[ENRICHMENT] Failed to fetch item %s after %d attempts: %v", id, attempt, err)
						enrichedData = &EnrichedItemData{
							ItemID:     id,
							EnrichedAt: time.Now(),
						}
						break
					}

					// Exponential backoff: 1s, 2s, 4s
					backoff := time.Duration(1<<(attempt-1)) * time.Second
					log.Printf("[ENRICHMENT] Retrying item %s in %v...", id, backoff)
					time.Sleep(backoff)
				}

				// Cache the result
				h.enrichmentMutex.Lock()
				h.enrichmentCache[id] = enrichedData
				h.enrichmentMutex.Unlock()

				// Add to result
				resultMutex.Lock()
				result[id] = *enrichedData
				resultMutex.Unlock()
			}(itemID)
		}

		wg.Wait()
		log.Printf("[ENRICHMENT] Completed fetching %d items", len(toFetch))
	}

	jsonResponse(w, http.StatusOK, result)
}

// GetFulfillmentPolicies returns shipping policies
func (h *Handler) GetFulfillmentPolicies(w http.ResponseWriter, r *http.Request) {
	client, err := h.getEbayClient(r)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Session error")
		return
	}

	if !client.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	marketplaceID := r.URL.Query().Get("marketplace_id")
	if marketplaceID == "" {
		marketplaceID = "EBAY_AU" // Default to eBay Australia
	}

	policies, err := client.GetFulfillmentPolicies(r.Context(), marketplaceID)
	if err != nil {
		log.Printf("GetFulfillmentPolicies error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, policies)
}

// CalculateRequest is the request body for calculate endpoint
type CalculateRequest struct {
	ItemValueAUD      float64 `json:"itemValueAUD"`
	WeightBand        string  `json:"weightBand"`
	BrandName         string  `json:"brandName"`
	CountryOfOrigin   string  `json:"countryOfOrigin,omitempty"`
	IncludeExtraCover bool    `json:"includeExtraCover"`
	DiscountBand      int     `json:"discountBand"`
}

// CalculateShipping calculates shipping costs
func (h *Handler) CalculateShipping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req CalculateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := calculator.CalculateUSAShipping(calculator.CalculateUSAShippingParams{
		ItemValueAUD:      req.ItemValueAUD,
		WeightBand:        req.WeightBand,
		BrandName:         req.BrandName,
		CountryOfOrigin:   req.CountryOfOrigin,
		IncludeExtraCover: req.IncludeExtraCover,
		DiscountBand:      req.DiscountBand,
	})
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

// GetBrands returns available brands
func (h *Handler) GetBrands(w http.ResponseWriter, r *http.Request) {
	brands := calculator.GetAvailableBrands()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"brands": brands,
		"total":  len(brands),
	})
}

// GetWeightBands returns available weight bands
func (h *Handler) GetWeightBands(w http.ResponseWriter, r *http.Request) {
	bands := calculator.GetWeightBands()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"weightBands": bands,
	})
}

// GetTariffCountries returns countries with tariff rates
func (h *Handler) GetTariffCountries(w http.ResponseWriter, r *http.Request) {
	countries := calculator.GetTariffCountries()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"countries": countries,
	})
}

// Reference Data CRUD Endpoints

// ReferenceTariffs handles CRUD operations for tariff rates
func (h *Handler) ReferenceTariffs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTariffs(w, r)
	case http.MethodPost:
		h.createTariff(w, r)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ReferenceTariffByID handles CRUD operations for a specific tariff rate
func (h *Handler) ReferenceTariffByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/reference/tariffs/:id
	idStr := r.URL.Path[len("/api/reference/tariffs/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid tariff ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.updateTariff(w, r, id)
	case http.MethodDelete:
		h.deleteTariff(w, r, id)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) listTariffs(w http.ResponseWriter, r *http.Request) {
	tariffs, err := h.db.GetAllTariffRates()
	if err != nil {
		log.Printf("Error fetching tariffs: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to fetch tariffs")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"tariffs": tariffs,
		"total":   len(tariffs),
	})
}

func (h *Handler) createTariff(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CountryName string  `json:"countryName"`
		TariffRate  float64 `json:"tariffRate"`
		Notes       string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CountryName == "" {
		errorResponse(w, http.StatusBadRequest, "Country name required")
		return
	}
	if req.TariffRate < 0 || req.TariffRate > 1 {
		errorResponse(w, http.StatusBadRequest, "Tariff rate must be between 0 and 1")
		return
	}

	id, err := h.db.CreateTariffRate(req.CountryName, req.TariffRate, req.Notes)
	if err != nil {
		log.Printf("Error creating tariff: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to create tariff")
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"message": "Tariff created successfully",
	})
}

func (h *Handler) updateTariff(w http.ResponseWriter, r *http.Request, id int64) {
	var req struct {
		CountryName string  `json:"countryName"`
		TariffRate  float64 `json:"tariffRate"`
		Notes       string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CountryName == "" {
		errorResponse(w, http.StatusBadRequest, "Country name required")
		return
	}
	if req.TariffRate < 0 || req.TariffRate > 1 {
		errorResponse(w, http.StatusBadRequest, "Tariff rate must be between 0 and 1")
		return
	}

	if err := h.db.UpdateTariffRate(id, req.CountryName, req.TariffRate, req.Notes); err != nil {
		log.Printf("Error updating tariff: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to update tariff")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Tariff updated successfully"})
}

func (h *Handler) deleteTariff(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.db.DeleteTariffRate(id); err != nil {
		log.Printf("Error deleting tariff: %v", err)
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Tariff deleted successfully"})
}

// ReferenceBrands handles CRUD operations for brand COO mappings
func (h *Handler) ReferenceBrands(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listBrands(w, r)
	case http.MethodPost:
		h.createBrand(w, r)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ReferenceBrandByID handles CRUD operations for a specific brand mapping
func (h *Handler) ReferenceBrandByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/reference/brands/:id
	idStr := r.URL.Path[len("/api/reference/brands/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid brand ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.updateBrand(w, r, id)
	case http.MethodDelete:
		h.deleteBrand(w, r, id)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) listBrands(w http.ResponseWriter, r *http.Request) {
	brands, err := h.db.GetAllBrandCOOMappings()
	if err != nil {
		log.Printf("Error fetching brands: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to fetch brands")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"brands": brands,
		"total":  len(brands),
	})
}

func (h *Handler) createBrand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BrandName  string `json:"brandName"`
		PrimaryCOO string `json:"primaryCoo"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.BrandName == "" {
		errorResponse(w, http.StatusBadRequest, "Brand name required")
		return
	}
	if req.PrimaryCOO == "" {
		errorResponse(w, http.StatusBadRequest, "Primary COO required")
		return
	}

	id, err := h.db.CreateBrandCOOMapping(req.BrandName, req.PrimaryCOO, req.Notes)
	if err != nil {
		log.Printf("Error creating brand: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to create brand")
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"message": "Brand created successfully",
	})
}

func (h *Handler) updateBrand(w http.ResponseWriter, r *http.Request, id int64) {
	var req struct {
		BrandName  string `json:"brandName"`
		PrimaryCOO string `json:"primaryCoo"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.BrandName == "" {
		errorResponse(w, http.StatusBadRequest, "Brand name required")
		return
	}
	if req.PrimaryCOO == "" {
		errorResponse(w, http.StatusBadRequest, "Primary COO required")
		return
	}

	if err := h.db.UpdateBrandCOOMapping(id, req.BrandName, req.PrimaryCOO, req.Notes); err != nil {
		log.Printf("Error updating brand: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to update brand")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Brand updated successfully"})
}

func (h *Handler) deleteBrand(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.db.DeleteBrandCOOMapping(id); err != nil {
		log.Printf("Error deleting brand: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to delete brand")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Brand deleted successfully"})
}

// UpdateShippingRequest is the request for updating shipping
type UpdateShippingRequest struct {
	OfferID   string                      `json:"offerId"`
	Overrides []ebay.ShippingCostOverride `json:"overrides"`
}

// UpdateOfferShipping updates shipping cost overrides
func (h *Handler) UpdateOfferShipping(w http.ResponseWriter, r *http.Request) {
	client, err := h.getEbayClient(r)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Session error")
		return
	}

	if !client.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req UpdateShippingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := client.UpdateOfferShipping(r.Context(), req.OfferID, req.Overrides); err != nil {
		log.Printf("UpdateOfferShipping error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "updated"})
}

// SyncExport exports current eBay account data to database
func (h *Handler) SyncExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	client, err := h.getEbayClient(r)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Session error")
		return
	}

	if !client.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	if h.currentAccount == nil {
		errorResponse(w, http.StatusBadRequest, "Not connected to an eBay account. Please authenticate first.")
		return
	}

	marketplaceID := r.URL.Query().Get("marketplace_id")
	if marketplaceID == "" {
		marketplaceID = h.currentAccount.MarketplaceID
	}

	log.Printf("Starting export for account: %s", h.currentAccount.DisplayName)

	err = h.syncService.ExportFromEbay(r.Context(), client, h.currentAccount.ID, marketplaceID)
	if err != nil {
		log.Printf("Export failed: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update last export time
	if err := h.db.UpdateLastExport(h.currentAccount.ID); err != nil {
		log.Printf("Failed to update last export time: %v", err)
	}

	log.Printf("Export completed successfully")
	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Exported data from " + h.currentAccount.DisplayName,
	})
}

// SyncImportRequest is the request body for import
type SyncImportRequest struct {
	SourceAccountKey string `json:"sourceAccountKey"` // Which account's data to import from
}

// SyncImport imports data from database to current eBay account
func (h *Handler) SyncImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	client, err := h.getEbayClient(r)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Session error")
		return
	}

	if !client.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	if h.currentAccount == nil {
		errorResponse(w, http.StatusBadRequest, "Not connected to an eBay account. Please authenticate first.")
		return
	}

	var req SyncImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get source account
	sourceAccount, err := h.db.GetAccountByKey(req.SourceAccountKey)
	if err != nil {
		log.Printf("Failed to get source account: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if sourceAccount == nil {
		errorResponse(w, http.StatusNotFound, "Source account not found: "+req.SourceAccountKey)
		return
	}

	log.Printf("Starting import from %s to %s", sourceAccount.DisplayName, h.currentAccount.DisplayName)

	err = h.syncService.ImportToEbay(r.Context(), client, sourceAccount.ID, h.currentAccount.ID)
	if err != nil {
		log.Printf("Import failed: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Import completed successfully")
	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Imported data from " + sourceAccount.DisplayName + " to " + h.currentAccount.DisplayName,
	})
}

// GetSyncHistory returns sync history
func (h *Handler) GetSyncHistory(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var history []database.SyncHistory
	var err error

	if h.currentAccount != nil {
		history, err = h.db.GetSyncHistory(h.currentAccount.ID, limit)
	} else {
		// If no current account, return empty
		history = []database.SyncHistory{}
	}

	if err != nil {
		log.Printf("GetSyncHistory error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"total":   len(history),
	})
}

// Simple state generator (in production, use crypto/rand)
func generateState() string {
	return "ebay-helpers-" + strconv.FormatInt(int64(100000+len("state")*12345), 36)
}

// MarketplaceAccountDeletion handles eBay marketplace account deletion notifications
// Required for production API credential activation
// Docs: https://developer.ebay.com/develop/guides-v2/marketplace-user-account-deletion
func (h *Handler) MarketplaceAccountDeletion(w http.ResponseWriter, r *http.Request) {
	// Handle GET request for endpoint validation
	if r.Method == http.MethodGet {
		h.handleDeletionValidation(w, r)
		return
	}

	// Handle POST request for actual deletion notifications
	if r.Method == http.MethodPost {
		h.handleDeletionNotification(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleDeletionValidation handles eBay's endpoint validation challenge
func (h *Handler) handleDeletionValidation(w http.ResponseWriter, r *http.Request) {
	challengeCode := r.URL.Query().Get("challenge_code")
	if challengeCode == "" {
		log.Printf("Deletion validation: missing challenge_code")
		http.Error(w, "Missing challenge_code parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Deletion validation challenge received: %s", challengeCode)

	// Compute SHA-256 hash: challengeCode + verificationToken + endpoint
	hashInput := challengeCode + h.verificationToken + h.endpoint
	hash := sha256.Sum256([]byte(hashInput))
	challengeResponse := hex.EncodeToString(hash[:])

	log.Printf("Computed challenge response: %s", challengeResponse)

	// Return JSON response with challenge response
	jsonResponse(w, http.StatusOK, map[string]string{
		"challengeResponse": challengeResponse,
	})
}

// EbayDeletionNotification represents the structure of eBay's deletion notification
type EbayDeletionNotification struct {
	Metadata struct {
		Topic         string `json:"topic"`
		SchemaVersion string `json:"schemaVersion"`
	} `json:"metadata"`
	Notification struct {
		NotificationID string `json:"notificationId"`
		EventDate      string `json:"eventDate"` // ISO 8601 format
		Data           struct {
			Username  string `json:"username"`
			UserID    string `json:"userId"`
			EiasToken string `json:"eiasToken"`
		} `json:"data"`
	} `json:"notification"`
}

// handleDeletionNotification handles actual account deletion notifications
func (h *Handler) handleDeletionNotification(w http.ResponseWriter, r *http.Request) {
	// Parse the notification payload
	var notification EbayDeletionNotification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		log.Printf("Failed to parse deletion notification: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	log.Printf("Received deletion notification for user: %s (ID: %s, Notification: %s)",
		notification.Notification.Data.Username,
		notification.Notification.Data.UserID,
		notification.Notification.NotificationID)

	// Parse event date
	eventDate, err := time.Parse(time.RFC3339, notification.Notification.EventDate)
	if err != nil {
		log.Printf("Failed to parse event date: %v", err)
		eventDate = time.Now() // Fallback to current time
	}

	// Convert back to JSON for storage
	rawPayload, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Failed to marshal notification for storage: %v", err)
		rawPayload = []byte("{}")
	}

	// Store the notification in database
	dn := &database.DeletionNotification{
		NotificationID: notification.Notification.NotificationID,
		Username:       notification.Notification.Data.Username,
		UserID:         notification.Notification.Data.UserID,
		EiasToken:      notification.Notification.Data.EiasToken,
		EventDate:      eventDate,
		RawPayload:     string(rawPayload),
	}

	if err := h.db.CreateDeletionNotification(dn); err != nil {
		log.Printf("Failed to store deletion notification: %v", err)
		// Still return success to eBay to avoid retries
	} else {
		log.Printf("Stored deletion notification: %s", dn.NotificationID)
	}

	// NOTE: This application uses memory-only OAuth token storage (tokens lost on restart).
	// No persistent user credentials are stored, so there is no user data to delete.
	// The notification is logged for eBay compliance and audit trail purposes.
	//
	// If OAuth token persistence is implemented in the future, token deletion logic
	// must be added here to match on notification.Notification.Data.UserID.

	log.Printf("Notification logged. No persistent user data to delete (memory-only OAuth tokens).")

	// Mark as processed immediately
	if err := h.db.MarkDeletionNotificationProcessed(dn.NotificationID); err != nil {
		log.Printf("Failed to mark notification as processed: %v", err)
	}

	// Respond with 200 OK (or 201/202/204 as per eBay docs)
	w.WriteHeader(http.StatusOK)
}

// GetDeletionNotifications returns deletion notifications for admin viewing
func (h *Handler) GetDeletionNotifications(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	notifications, err := h.db.GetDeletionNotifications(limit)
	if err != nil {
		log.Printf("GetDeletionNotifications error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"total":         len(notifications),
	})
}

// BatchCalculateRequest holds items for batch calculation
type BatchCalculateItem struct {
	ItemID string  `json:"itemId"`
	Price  float64 `json:"price"`
}

// BatchCalculateResponse holds calculated data for an item
type BatchCalculateResponse struct {
	ItemID         string  `json:"itemId"`
	ExpectedCOO    string  `json:"expectedCoo"`
	COOStatus      string  `json:"cooStatus"` // "match", "mismatch", "missing"
	CalculatedCost float64 `json:"calculatedCost"`
	Diff           float64 `json:"diff"`
	DiffStatus     string  `json:"diffStatus"` // "ok" or "bad"
}

// BatchCalculate calculates postage for multiple items using server-side logic
// Frontend sends item IDs + prices, backend returns calculated costs
// This keeps business logic on backend while allowing frontend to display results
func (h *Handler) BatchCalculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var items []BatchCalculateItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results := make(map[string]BatchCalculateResponse)

	for _, item := range items {
		// Get enrichment data from cache (brand, COO, shipping)
		h.enrichmentMutex.RLock()
		enriched, exists := h.enrichmentCache[item.ItemID]
		h.enrichmentMutex.RUnlock()

		if !exists || enriched == nil {
			continue // Skip items not yet enriched
		}

		// Get expected COO from brand mapping
		expectedCOO := calculator.GetCountryOfOrigin(enriched.Brand)

		// Determine COO status
		var cooStatus string
		coo := enriched.CountryOfOrigin
		if coo == "" {
			cooStatus = "missing"
			coo = expectedCOO // Use expected for calculation
		} else if coo == expectedCOO {
			cooStatus = "match"
		} else {
			cooStatus = "mismatch"
		}

		// Calculate postage using backend calculator
		result, err := calculator.CalculateUSAShipping(calculator.CalculateUSAShippingParams{
			ItemValueAUD:      item.Price,
			WeightBand:        "Medium", // Default - TODO: make configurable
			BrandName:         enriched.Brand,
			CountryOfOrigin:   coo,
			IncludeExtraCover: item.Price > 100,
			DiscountBand:      3, // Default band 3 - TODO: make configurable
		})

		if err != nil {
			log.Printf("[BATCH-CALC] Error calculating item %s: %v", item.ItemID, err)
			continue
		}

		// Calculate diff
		shippingCost := 0.0
		if enriched.ShippingCost != "" {
			fmt.Sscanf(enriched.ShippingCost, "%f", &shippingCost)
		}
		diff := shippingCost - result.Total

		// Determine diff status (5% threshold)
		var diffStatus string
		threshold := result.Total * 1.05
		if shippingCost >= threshold {
			diffStatus = "ok"
		} else {
			diffStatus = "bad"
		}

		results[item.ItemID] = BatchCalculateResponse{
			ItemID:         item.ItemID,
			ExpectedCOO:    expectedCOO,
			COOStatus:      cooStatus,
			CalculatedCost: result.Total,
			Diff:           diff,
			DiffStatus:     diffStatus,
		}
	}

	jsonResponse(w, http.StatusOK, results)
}

// GetListings returns enriched listings from database with server-side sort/filter/pagination
// This is the proper backend-driven approach - frontend just renders what API returns
func (h *Handler) GetListings(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := database.ListingsQuery{
		Search:    r.URL.Query().Get("search"),
		SortBy:    r.URL.Query().Get("sort"),
		SortOrder: r.URL.Query().Get("order"),
	}

	// Parse page number
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			query.Page = page
		}
	}

	// Parse page size
	query.PageSize = 50 // Default
	if sizeStr := r.URL.Query().Get("pageSize"); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 && size <= 100 {
			query.PageSize = size
		}
	}

	// Query database
	result, err := h.db.GetListings(query)
	if err != nil {
		log.Printf("GetListings error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}
