package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/julienbonastre/ebay-helpers/internal/calculator"
	"github.com/julienbonastre/ebay-helpers/internal/database"
	"github.com/julienbonastre/ebay-helpers/internal/ebay"
	"github.com/julienbonastre/ebay-helpers/internal/sync"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	db             *database.DB
	ebayClient     *ebay.Client
	currentAccount *database.Account // Current instance's account (can be nil)
	syncService    *sync.Service
	mu             sync.RWMutex
	oauthState     string
}

// NewHandler creates a new handler
func NewHandler(db *database.DB, client *ebay.Client, currentAccount *database.Account) *Handler {
	return &Handler{
		db:             db,
		ebayClient:     client,
		currentAccount: currentAccount,
		syncService:    sync.NewService(db),
	}
}

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
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"authenticated": h.ebayClient.IsAuthenticated(),
		"configured":    h.ebayClient.IsConfigured(),
		"hasAccount":    h.currentAccount != nil,
	})
}

// GetCurrentAccount returns the current instance's account info
func (h *Handler) GetCurrentAccount(w http.ResponseWriter, r *http.Request) {
	if h.currentAccount == nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"configured": false,
			"message":    "No account specified. Use -store flag when starting server.",
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"configured": true,
		"account":    h.currentAccount,
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

	url := h.ebayClient.GetAuthURL(state)
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
	if err := h.ebayClient.ExchangeCode(r.Context(), code); err != nil {
		log.Printf("OAuth exchange error: %v", err)
		http.Error(w, "Failed to authenticate: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("OAuth success! Token obtained.")
	// Redirect to the main app
	http.Redirect(w, r, "/?auth=success", http.StatusFound)
}

// GetAuthStatus returns current auth status
func (h *Handler) GetAuthStatus(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"authenticated": h.ebayClient.IsAuthenticated(),
		"configured":    h.ebayClient.IsConfigured(),
	})
}

// GetInventoryItems returns paginated inventory items
func (h *Handler) GetInventoryItems(w http.ResponseWriter, r *http.Request) {
	if !h.ebayClient.IsAuthenticated() {
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

	items, err := h.ebayClient.GetInventoryItems(r.Context(), limit, offset)
	if err != nil {
		log.Printf("GetInventoryItems error: %v", err)
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, items)
}

// GetOffers returns paginated offers
func (h *Handler) GetOffers(w http.ResponseWriter, r *http.Request) {
	if !h.ebayClient.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	sku := r.URL.Query().Get("sku")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}

	offers, err := h.ebayClient.GetOffers(r.Context(), sku, limit, offset)
	if err != nil {
		log.Printf("GetOffers error: %v", err)
		// Check if it's a SKU validation error (likely empty inventory)
		if strings.Contains(err.Error(), "invalid value for a SKU") || strings.Contains(err.Error(), "25707") {
			// Return empty result instead of error
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"offers": []interface{}{},
				"total":  0,
				"limit":  limit,
				"offset": offset,
			})
			return
		}
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, offers)
}

// GetFulfillmentPolicies returns shipping policies
func (h *Handler) GetFulfillmentPolicies(w http.ResponseWriter, r *http.Request) {
	if !h.ebayClient.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	marketplaceID := r.URL.Query().Get("marketplace_id")
	if marketplaceID == "" {
		marketplaceID = "EBAY_AU" // Default to eBay Australia
	}

	policies, err := h.ebayClient.GetFulfillmentPolicies(r.Context(), marketplaceID)
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

// UpdateShippingRequest is the request for updating shipping
type UpdateShippingRequest struct {
	OfferID   string                      `json:"offerId"`
	Overrides []ebay.ShippingCostOverride `json:"overrides"`
}

// UpdateOfferShipping updates shipping cost overrides
func (h *Handler) UpdateOfferShipping(w http.ResponseWriter, r *http.Request) {
	if !h.ebayClient.IsAuthenticated() {
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

	if err := h.ebayClient.UpdateOfferShipping(r.Context(), req.OfferID, req.Overrides); err != nil {
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

	if !h.ebayClient.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	if h.currentAccount == nil {
		errorResponse(w, http.StatusBadRequest, "No account configured. Restart with -store flag.")
		return
	}

	marketplaceID := r.URL.Query().Get("marketplace_id")
	if marketplaceID == "" {
		marketplaceID = h.currentAccount.MarketplaceID
	}

	log.Printf("Starting export for account: %s", h.currentAccount.DisplayName)

	err := h.syncService.ExportFromEbay(r.Context(), h.ebayClient, h.currentAccount.ID, marketplaceID)
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

	if !h.ebayClient.IsAuthenticated() {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated with eBay")
		return
	}

	if h.currentAccount == nil {
		errorResponse(w, http.StatusBadRequest, "No account configured. Restart with -store flag.")
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

	err = h.syncService.ImportToEbay(r.Context(), h.ebayClient, sourceAccount.ID, h.currentAccount.ID)
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
