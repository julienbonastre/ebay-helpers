package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/julienbonastre/ebay-helpers/internal/calculator"
	"github.com/julienbonastre/ebay-helpers/internal/ebay"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	ebayClient *ebay.Client
	mu         sync.RWMutex
	oauthState string
}

// NewHandler creates a new handler
func NewHandler(client *ebay.Client) *Handler {
	return &Handler{
		ebayClient: client,
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
	})
}

// GetAuthURL returns the eBay OAuth URL
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
	OfferID   string                    `json:"offerId"`
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

// Simple state generator (in production, use crypto/rand)
func generateState() string {
	return "ebay-helpers-" + strconv.FormatInt(int64(100000+len("state")*12345), 36)
}
