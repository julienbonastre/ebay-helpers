package ebay

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	// Sandbox URLs
	SandboxAuthURL          = "https://auth.sandbox.ebay.com/oauth2/authorize"
	SandboxTokenURL         = "https://api.sandbox.ebay.com/identity/v1/oauth2/token"
	SandboxAPIBaseURL       = "https://api.sandbox.ebay.com"        // For Sell APIs
	SandboxCommerceBaseURL  = "https://apiz.sandbox.ebay.com"       // For Commerce APIs
	SandboxTradingAPIURL    = "https://api.sandbox.ebay.com/ws/api.dll" // For Trading API (XML)

	// Production URLs
	ProductionAuthURL         = "https://auth.ebay.com/oauth2/authorize"
	ProductionTokenURL        = "https://api.ebay.com/identity/v1/oauth2/token"
	ProductionAPIBaseURL      = "https://api.ebay.com"                // For Sell APIs
	ProductionCommerceBaseURL = "https://apiz.ebay.com"             // For Commerce APIs (note the 'z')
	ProductionTradingAPIURL   = "https://api.ebay.com/ws/api.dll"   // For Trading API (XML)
)

// Config holds eBay API configuration
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Sandbox      bool
	Scopes       []string
}

// Client is the eBay API client
type Client struct {
	config          Config
	httpClient      *http.Client
	oauthConfig     *oauth2.Config
	token           *oauth2.Token
	baseURL         string  // For Sell APIs (api.ebay.com)
	commerceBaseURL string  // For Commerce APIs (apiz.ebay.com)
	tradingAPIURL   string  // For Trading API (XML-based)
}

// NewClient creates a new eBay API client
func NewClient(cfg Config) *Client {
	var authURL, tokenURL, baseURL, commerceBaseURL, tradingAPIURL string
	if cfg.Sandbox {
		authURL = SandboxAuthURL
		tokenURL = SandboxTokenURL
		baseURL = SandboxAPIBaseURL
		commerceBaseURL = SandboxCommerceBaseURL
		tradingAPIURL = SandboxTradingAPIURL
	} else {
		authURL = ProductionAuthURL
		tokenURL = ProductionTokenURL
		baseURL = ProductionAPIBaseURL
		commerceBaseURL = ProductionCommerceBaseURL
		tradingAPIURL = ProductionTradingAPIURL
	}

	// Default scopes for inventory management
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{
			"https://api.ebay.com/oauth/api_scope",
			"https://api.ebay.com/oauth/api_scope/sell.inventory",
			"https://api.ebay.com/oauth/api_scope/sell.inventory.readonly",
			"https://api.ebay.com/oauth/api_scope/sell.account",
			"https://api.ebay.com/oauth/api_scope/sell.account.readonly",
			"https://api.ebay.com/oauth/api_scope/sell.fulfillment",
			"https://api.ebay.com/oauth/api_scope/sell.fulfillment.readonly",
			"https://api.ebay.com/oauth/api_scope/commerce.identity.readonly", // For User API
		}
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}

	return &Client{
		config:          cfg,
		oauthConfig:     oauthConfig,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		baseURL:         baseURL,
		commerceBaseURL: commerceBaseURL,
		tradingAPIURL:   tradingAPIURL,
	}
}

// GetAuthURL returns the OAuth authorization URL
func (c *Client) GetAuthURL(state string) string {
	// eBay uses "prompt=login" to force re-authentication
	// Note: eBay automatically provides refresh tokens, no access_type needed
	url := c.oauthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("prompt", "login"))

	// DEBUG: Log the full OAuth URL (with truncated state for security)
	log.Printf("[OAUTH-DEBUG] Generated OAuth URL: %s", url)
	log.Printf("[OAUTH-DEBUG] Redirect URI: %s", c.oauthConfig.RedirectURL)
	log.Printf("[OAUTH-DEBUG] Client ID: %s", c.oauthConfig.ClientID[:15]+"...")
	log.Printf("[OAUTH-DEBUG] Scopes: %v", c.oauthConfig.Scopes)

	return url
}

// ExchangeCode exchanges an auth code for tokens
func (c *Client) ExchangeCode(ctx context.Context, code string) error {
	log.Printf("[OAUTH-DEBUG] Exchanging authorization code (first 20 chars): %s...", code[:min(20, len(code))])
	log.Printf("[OAUTH-DEBUG] Token URL: %s", c.oauthConfig.Endpoint.TokenURL)

	token, err := c.oauthConfig.Exchange(ctx, code)
	if err != nil {
		log.Printf("[OAUTH-ERROR] Token exchange failed: %v", err)
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	// DEBUG: Log token details (NOT the actual token value for security)
	log.Printf("[OAUTH-DEBUG] Token exchange successful!")
	log.Printf("[OAUTH-DEBUG] Token type: %s", token.TokenType)
	log.Printf("[OAUTH-DEBUG] Token expiry: %v", token.Expiry)
	log.Printf("[OAUTH-DEBUG] Token expires in: %v", time.Until(token.Expiry))
	log.Printf("[OAUTH-DEBUG] Has refresh token: %v", token.RefreshToken != "")

	// Try to extract scopes from the token if available
	if scopes, ok := token.Extra("scope").(string); ok {
		log.Printf("[OAUTH-DEBUG] Token scopes: %s", scopes)
	} else {
		log.Printf("[OAUTH-DEBUG] No scope information in token response")
	}

	c.token = token
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetToken sets the OAuth token directly
func (c *Client) SetToken(token *oauth2.Token) {
	c.token = token
}

// GetToken returns the current token
func (c *Client) GetToken() *oauth2.Token {
	return c.token
}

// IsAuthenticated returns true if we have a valid token
func (c *Client) IsAuthenticated() bool {
	return c.token != nil && c.token.Valid()
}

// IsConfigured returns true if eBay API credentials are set
func (c *Client) IsConfigured() bool {
	return c.config.ClientID != "" && c.config.ClientSecret != ""
}

// RefreshToken refreshes the access token if needed
func (c *Client) RefreshToken(ctx context.Context) error {
	if c.token == nil {
		return fmt.Errorf("no token to refresh")
	}

	src := c.oauthConfig.TokenSource(ctx, c.token)
	newToken, err := src.Token()
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	c.token = newToken
	return nil
}

// doRequest makes an authenticated API request (for Sell APIs)
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("client not authenticated")
	}

	// Ensure token is fresh
	src := c.oauthConfig.TokenSource(ctx, c.token)
	token, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}
	c.token = token

	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// doCommerceRequest makes an authenticated API request (for Commerce APIs using apiz.ebay.com)
func (c *Client) doCommerceRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("client not authenticated")
	}

	// Ensure token is fresh
	src := c.oauthConfig.TokenSource(ctx, c.token)
	token, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}
	c.token = token

	reqURL := c.commerceBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// User represents an eBay user
type User struct {
	UserID       string `json:"userId"`       // Immutable user ID
	Username     string `json:"username"`     // eBay username
	Email        string `json:"email"`        // User's email (if available)
	FirstName    string `json:"firstName"`    // First name
	LastName     string `json:"lastName"`     // Last name
	MarketplaceID string `json:"marketplaceId"` // Primary marketplace
}

// GetUser fetches the authenticated user's information
func (c *Client) GetUser(ctx context.Context) (*User, error) {
	// Commerce APIs use apiz.ebay.com not api.ebay.com
	fullURL := c.commerceBaseURL + "/commerce/identity/v1/user/"
	log.Printf("[USER-API-DEBUG] Calling User API: GET %s", fullURL)
	log.Printf("[USER-API-DEBUG] Has token: %v, Token valid: %v", c.token != nil, c.token != nil && c.token.Valid())

	// Call Commerce API directly (uses different base URL than Sell APIs)
	resp, err := c.doCommerceRequest(ctx, "GET", "/commerce/identity/v1/user/", nil)
	if err != nil {
		log.Printf("[USER-API-ERROR] doRequest failed: %v", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[USER-API-DEBUG] Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[USER-API-ERROR] Non-200 response body: %s", string(body))
		return nil, fmt.Errorf("user API returned status %d: %s", resp.StatusCode, string(body))
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Printf("[USER-API-ERROR] Failed to decode response: %v", err)
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	log.Printf("[USER-API-DEBUG] Successfully retrieved user: %s (ID: %s)", user.Username, user.UserID)
	return &user, nil
}

// InventoryItem represents an eBay inventory item
type InventoryItem struct {
	SKU         string            `json:"sku"`
	Locale      string            `json:"locale,omitempty"`
	Product     *Product          `json:"product,omitempty"`
	Condition   string            `json:"condition,omitempty"`
	Availability *Availability    `json:"availability,omitempty"`
}

// Product holds product details
type Product struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	ImageURLs   []string `json:"imageUrls,omitempty"`
	Brand       string   `json:"brand,omitempty"`
}

// Availability holds inventory availability
type Availability struct {
	ShipToLocationAvailability *ShipToLocation `json:"shipToLocationAvailability,omitempty"`
}

// ShipToLocation holds quantity info
type ShipToLocation struct {
	Quantity int `json:"quantity,omitempty"`
}

// Offer represents an eBay listing offer
type Offer struct {
	OfferID             string              `json:"offerId,omitempty"`
	SKU                 string              `json:"sku,omitempty"`
	MarketplaceID       string              `json:"marketplaceId,omitempty"`
	Format              string              `json:"format,omitempty"`
	ListingDescription  string              `json:"listingDescription,omitempty"`
	PricingSummary      *PricingSummary     `json:"pricingSummary,omitempty"`
	ListingPolicies     *ListingPolicies    `json:"listingPolicies,omitempty"`
	Status              string              `json:"status,omitempty"`
	Listing             *ListingDetails     `json:"listing,omitempty"`
}

// PricingSummary holds pricing info
type PricingSummary struct {
	Price *Amount `json:"price,omitempty"`
}

// Amount holds monetary values
type Amount struct {
	Value    string `json:"value,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// ListingPolicies holds policy references
type ListingPolicies struct {
	FulfillmentPolicyID    string                  `json:"fulfillmentPolicyId,omitempty"`
	PaymentPolicyID        string                  `json:"paymentPolicyId,omitempty"`
	ReturnPolicyID         string                  `json:"returnPolicyId,omitempty"`
	ShippingCostOverrides  []ShippingCostOverride  `json:"shippingCostOverrides,omitempty"`
}

// ShippingCostOverride allows overriding shipping costs
type ShippingCostOverride struct {
	ShippingServiceType    string  `json:"shippingServiceType,omitempty"` // DOMESTIC or INTERNATIONAL
	Priority               int     `json:"priority,omitempty"`
	ShippingCost           *Amount `json:"shippingCost,omitempty"`
	AdditionalShippingCost *Amount `json:"additionalShippingCost,omitempty"`
}

// ListingDetails holds listing info
type ListingDetails struct {
	ListingID string `json:"listingId,omitempty"`
}

// OffersResponse is the response from getOffers
type OffersResponse struct {
	Offers []Offer `json:"offers,omitempty"`
	Total  int     `json:"total,omitempty"`
	Limit  int     `json:"limit,omitempty"`
	Offset int     `json:"offset,omitempty"`
	Href   string  `json:"href,omitempty"`
	Next   string  `json:"next,omitempty"`
}

// InventoryItemsResponse is the response from getInventoryItems
type InventoryItemsResponse struct {
	InventoryItems []InventoryItem `json:"inventoryItems,omitempty"`
	Total          int             `json:"total,omitempty"`
	Limit          int             `json:"limit,omitempty"`
	Offset         int             `json:"offset,omitempty"`
	Href           string          `json:"href,omitempty"`
	Next           string          `json:"next,omitempty"`
}

// FulfillmentPolicy represents a shipping/fulfillment policy
type FulfillmentPolicy struct {
	FulfillmentPolicyID string           `json:"fulfillmentPolicyId,omitempty"`
	Name                string           `json:"name,omitempty"`
	MarketplaceID       string           `json:"marketplaceId,omitempty"`
	ShippingOptions     []ShippingOption `json:"shippingOptions,omitempty"`
}

// ShippingOption holds shipping option details
type ShippingOption struct {
	OptionType       string            `json:"optionType,omitempty"` // DOMESTIC or INTERNATIONAL
	ShippingServices []ShippingService `json:"shippingServices,omitempty"`
}

// ShippingService holds service details
type ShippingService struct {
	SortOrderID      int     `json:"sortOrderId,omitempty"`
	ShippingCarrier  string  `json:"shippingCarrierCode,omitempty"`
	ShippingService  string  `json:"shippingServiceCode,omitempty"`
	ShippingCost     *Amount `json:"shippingCost,omitempty"`
	AdditionalCost   *Amount `json:"additionalShippingCost,omitempty"`
	FreeShipping     bool    `json:"freeShipping,omitempty"`
	ShipToLocations  *ShipToLocations `json:"shipToLocations,omitempty"`
}

// ShipToLocations holds destination info
type ShipToLocations struct {
	RegionIncluded []Region `json:"regionIncluded,omitempty"`
	RegionExcluded []Region `json:"regionExcluded,omitempty"`
}

// Region represents a geographic region
type Region struct {
	RegionName string `json:"regionName,omitempty"`
	RegionType string `json:"regionType,omitempty"`
}

// FulfillmentPoliciesResponse is the response from getFulfillmentPolicies
type FulfillmentPoliciesResponse struct {
	FulfillmentPolicies []FulfillmentPolicy `json:"fulfillmentPolicies,omitempty"`
	Total               int                 `json:"total,omitempty"`
}

// PaymentPolicy represents a payment policy
type PaymentPolicy struct {
	PaymentPolicyID string          `json:"paymentPolicyId,omitempty"`
	Name            string          `json:"name,omitempty"`
	MarketplaceID   string          `json:"marketplaceId,omitempty"`
	PaymentMethods  []PaymentMethod `json:"paymentMethods,omitempty"`
	ImmediatePay    bool            `json:"immediatePay,omitempty"`
}

// PaymentMethod holds payment method details
type PaymentMethod struct {
	PaymentMethodType string `json:"paymentMethodType,omitempty"`
}

// PaymentPoliciesResponse is the response from getPaymentPolicies
type PaymentPoliciesResponse struct {
	PaymentPolicies []PaymentPolicy `json:"paymentPolicies,omitempty"`
	Total           int             `json:"total,omitempty"`
}

// ReturnPolicy represents a return policy
type ReturnPolicy struct {
	ReturnPolicyID           string       `json:"returnPolicyId,omitempty"`
	Name                     string       `json:"name,omitempty"`
	MarketplaceID            string       `json:"marketplaceId,omitempty"`
	ReturnsAccepted          bool         `json:"returnsAccepted,omitempty"`
	ReturnPeriod             *TimeDuration `json:"returnPeriod,omitempty"`
	ReturnShippingCostPayer  string       `json:"returnShippingCostPayer,omitempty"`
}

// TimeDuration represents a time duration
type TimeDuration struct {
	Value int    `json:"value,omitempty"`
	Unit  string `json:"unit,omitempty"` // "DAY", "MONTH"
}

// ReturnPoliciesResponse is the response from getReturnPolicies
type ReturnPoliciesResponse struct {
	ReturnPolicies []ReturnPolicy `json:"returnPolicies,omitempty"`
	Total          int            `json:"total,omitempty"`
}

// GetInventoryItems retrieves all inventory items
func (c *Client) GetInventoryItems(ctx context.Context, limit, offset int) (*InventoryItemsResponse, error) {
	path := fmt.Sprintf("/sell/inventory/v1/inventory_item?limit=%d&offset=%d", limit, offset)

	log.Printf("[INVENTORY-DEBUG] Fetching inventory: path=%s", path)
	log.Printf("[INVENTORY-DEBUG] Full URL: %s%s", c.baseURL, path)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		log.Printf("[INVENTORY-ERROR] doRequest failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("[INVENTORY-DEBUG] Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[INVENTORY-ERROR] Non-200 response: %s", string(body))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Read and log the raw response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[INVENTORY-ERROR] Failed to read response: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	log.Printf("[INVENTORY-DEBUG] Raw response: %s", string(body))

	var result InventoryItemsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[INVENTORY-ERROR] Failed to decode: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[INVENTORY-DEBUG] Successfully fetched %d items (total: %d)", len(result.InventoryItems), result.Total)
	return &result, nil
}

// GetOffers retrieves offers for a SKU or all offers
func (c *Client) GetOffers(ctx context.Context, sku string, limit, offset int) (*OffersResponse, error) {
	path := fmt.Sprintf("/sell/inventory/v1/offer?limit=%d&offset=%d", limit, offset)
	if sku != "" {
		path += "&sku=" + url.QueryEscape(sku)
	}

	log.Printf("[OFFERS-DEBUG] Fetching offers: path=%s, sku=%q, limit=%d, offset=%d", path, sku, limit, offset)
	log.Printf("[OFFERS-DEBUG] Full URL: %s%s", c.baseURL, path)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		log.Printf("[OFFERS-ERROR] doRequest failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("[OFFERS-DEBUG] Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[OFFERS-ERROR] Non-200 response: %s", string(body))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result OffersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[OFFERS-ERROR] Failed to decode: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[OFFERS-DEBUG] Successfully fetched %d offers (total: %d)", len(result.Offers), result.Total)
	return &result, nil
}

// GetFulfillmentPolicies retrieves all fulfillment policies
func (c *Client) GetFulfillmentPolicies(ctx context.Context, marketplaceID string) (*FulfillmentPoliciesResponse, error) {
	path := "/sell/account/v1/fulfillment_policy?marketplace_id=" + url.QueryEscape(marketplaceID)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result FulfillmentPoliciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GetPaymentPolicies retrieves all payment policies
func (c *Client) GetPaymentPolicies(ctx context.Context, marketplaceID string) (*PaymentPoliciesResponse, error) {
	path := "/sell/account/v1/payment_policy?marketplace_id=" + url.QueryEscape(marketplaceID)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result PaymentPoliciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GetReturnPolicies retrieves all return policies
func (c *Client) GetReturnPolicies(ctx context.Context, marketplaceID string) (*ReturnPoliciesResponse, error) {
	path := "/sell/account/v1/return_policy?marketplace_id=" + url.QueryEscape(marketplaceID)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result ReturnPoliciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// UpdateOfferShipping updates shipping cost overrides for an offer
func (c *Client) UpdateOfferShipping(ctx context.Context, offerID string, overrides []ShippingCostOverride) error {
	// First get the current offer
	path := "/sell/inventory/v1/offer/" + url.PathEscape(offerID)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get offer: %d %s", resp.StatusCode, string(body))
	}

	var offer Offer
	if err := json.NewDecoder(resp.Body).Decode(&offer); err != nil {
		return fmt.Errorf("failed to decode offer: %w", err)
	}

	// Update the shipping cost overrides
	if offer.ListingPolicies == nil {
		offer.ListingPolicies = &ListingPolicies{}
	}
	offer.ListingPolicies.ShippingCostOverrides = overrides

	// PUT the updated offer
	updateBody, err := json.Marshal(offer)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	resp2, err := c.doRequest(ctx, http.MethodPut, path, strings.NewReader(string(updateBody)))
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("failed to update offer: %d %s", resp2.StatusCode, string(body))
	}

	return nil
}

// TradingItem represents an item from GetMyeBaySelling (simplified)
type TradingItem struct {
	ItemID          string
	SKU             string
	Title           string
	Price           string
	Currency        string
	Quantity        int
	QuantitySold    int
	ImageURL        string
	Brand           string
	ShippingCost    string
	ShippingCurrency string
}

// XML response structures for GetMyeBaySelling
type GetMyeBaySellingResponse struct {
	XMLName    xml.Name `xml:"GetMyeBaySellingResponse"`
	Ack        string   `xml:"Ack"`
	ActiveList struct {
		ItemArray struct {
			Items []struct {
				ItemID        string `xml:"ItemID"`
				SKU           string `xml:"SKU"`
				Title         string `xml:"Title"`
				Quantity      int    `xml:"Quantity"`
				PictureDetails struct {
					GalleryURL    string `xml:"GalleryURL"`
					PictureURL    []string `xml:"PictureURL"`
				} `xml:"PictureDetails"`
				ItemSpecifics struct {
					NameValueList []struct {
						Name  string `xml:"Name"`
						Value string `xml:"Value"`
					} `xml:"NameValueList"`
				} `xml:"ItemSpecifics"`
				ShippingDetails struct {
					ShippingServiceOptions []struct {
						ShippingServiceCost struct {
							Value      string `xml:",chardata"`
							CurrencyID string `xml:"currencyID,attr"`
						} `xml:"ShippingServiceCost"`
					} `xml:"ShippingServiceOptions"`
					InternationalShippingServiceOption []struct {
						ShippingServiceCost struct {
							Value      string `xml:",chardata"`
							CurrencyID string `xml:"currencyID,attr"`
						} `xml:"ShippingServiceCost"`
						ShipToLocation []string `xml:"ShipToLocation"`
					} `xml:"InternationalShippingServiceOption"`
				} `xml:"ShippingDetails"`
				SellingStatus struct {
					CurrentPrice struct {
						Value      string `xml:",chardata"`
						CurrencyID string `xml:"currencyID,attr"`
					} `xml:"CurrentPrice"`
					QuantitySold int `xml:"QuantitySold"`
				} `xml:"SellingStatus"`
			} `xml:"Item"`
		} `xml:"ItemArray"`
		PaginationResult struct {
			TotalNumberOfPages   int `xml:"TotalNumberOfPages"`
			TotalNumberOfEntries int `xml:"TotalNumberOfEntries"`
		} `xml:"PaginationResult"`
	} `xml:"ActiveList"`
	Errors []struct {
		ShortMessage string `xml:"ShortMessage"`
		LongMessage  string `xml:"LongMessage"`
		ErrorCode    string `xml:"ErrorCode"`
	} `xml:"Errors>Error"`
}

// GetItemResponse represents the XML response from GetItem
type GetItemResponse struct {
	XMLName xml.Name `xml:"GetItemResponse"`
	Ack     string   `xml:"Ack"`
	Item    struct {
		ItemID          string `xml:"ItemID"`
		ItemSpecifics   struct {
			NameValueList []struct {
				Name  string `xml:"Name"`
				Value string `xml:"Value"`
			} `xml:"NameValueList"`
		} `xml:"ItemSpecifics"`
		PictureDetails struct {
			PictureURL []string `xml:"PictureURL"`
		} `xml:"PictureDetails"`
		ShippingDetails struct {
			ShippingServiceOptions []struct {
				ShippingServiceCost struct {
					Value      string `xml:",chardata"`
					CurrencyID string `xml:"currencyID,attr"`
				} `xml:"ShippingServiceCost"`
			} `xml:"ShippingServiceOptions"`
			InternationalShippingServiceOption []struct {
				ShippingServiceCost struct {
					Value      string `xml:",chardata"`
					CurrencyID string `xml:"currencyID,attr"`
				} `xml:"ShippingServiceCost"`
				ShipToLocation []string `xml:"ShipToLocation"`
			} `xml:"InternationalShippingServiceOption"`
		} `xml:"ShippingDetails"`
	} `xml:"Item"`
	Errors []struct {
		ShortMessage string `xml:"ShortMessage"`
		LongMessage  string `xml:"LongMessage"`
		ErrorCode    string `xml:"ErrorCode"`
	} `xml:"Errors>Error"`
}

// GetItem fetches full details for a single item by ItemID
func (c *Client) GetItem(ctx context.Context, itemID string) (brand, shippingCost, shippingCurrency, coo string, images []string, err error) {
	if !c.IsAuthenticated() {
		return "", "", "", "", nil, fmt.Errorf("client not authenticated")
	}

	// Ensure token is fresh
	src := c.oauthConfig.TokenSource(ctx, c.token)
	token, err := src.Token()
	if err != nil {
		return "", "", "", "", nil, fmt.Errorf("failed to get valid token: %w", err)
	}
	c.token = token

	// Build XML request for GetItem
	xmlRequest := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<GetItemRequest xmlns="urn:ebay:apis:eBLBaseComponents">
  <ItemID>%s</ItemID>
  <DetailLevel>ReturnAll</DetailLevel>
  <IncludeItemSpecifics>true</IncludeItemSpecifics>
</GetItemRequest>`, itemID)

	log.Printf("[GET-ITEM-DEBUG] Fetching item %s", itemID)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.tradingAPIURL, strings.NewReader(xmlRequest))
	if err != nil {
		return "", "", "", "", nil, err
	}

	// Set headers for Trading API
	req.Header.Set("X-EBAY-API-COMPATIBILITY-LEVEL", "967")
	req.Header.Set("X-EBAY-API-CALL-NAME", "GetItem")
	req.Header.Set("X-EBAY-API-SITEID", "15") // Australia
	req.Header.Set("X-EBAY-API-IAF-TOKEN", token.AccessToken)
	req.Header.Set("Content-Type", "text/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[GET-ITEM-ERROR] Request failed for item %s: %v", itemID, err)
		return "", "", "", "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", "", nil, err
	}

	// Parse XML response
	var xmlResp GetItemResponse
	if err := xml.Unmarshal(body, &xmlResp); err != nil {
		log.Printf("[GET-ITEM-ERROR] Failed to parse XML for item %s: %v", itemID, err)
		return "", "", "", "", nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Check for API errors
	if xmlResp.Ack != "Success" && xmlResp.Ack != "Warning" {
		if len(xmlResp.Errors) > 0 {
			errMsg := fmt.Sprintf("eBay API error %s: %s", xmlResp.Errors[0].ErrorCode, xmlResp.Errors[0].LongMessage)
			log.Printf("[GET-ITEM-ERROR] %s", errMsg)
			return "", "", "", "", nil, fmt.Errorf(errMsg)
		}
		return "", "", "", "", nil, fmt.Errorf("API returned Ack=%s", xmlResp.Ack)
	}

	// Extract Brand and Country of Origin from ItemSpecifics
	// Log all specs for debugging COO detection issues
	var allSpecNames []string
	for _, spec := range xmlResp.Item.ItemSpecifics.NameValueList {
		allSpecNames = append(allSpecNames, spec.Name)

		if spec.Name == "Brand" {
			brand = spec.Value
			log.Printf("[GET-ITEM-DEBUG] Item %s: Brand = %s", itemID, brand)
		}
		// Look for Country of Origin (can be stored as various names in eBay)
		// Common field names: "Country/Region of Manufacture", "Country of Manufacture", "Country of Origin"
		if spec.Name == "Country/Region of Manufacture" ||
		   spec.Name == "Country of Manufacture" ||
		   spec.Name == "Country of Origin" ||
		   spec.Name == "Country/Region of Origin" {
			coo = spec.Value
			log.Printf("[GET-ITEM-DEBUG] Item %s: Country of Origin = %s (field: %s)", itemID, coo, spec.Name)
		}
	}
	// If COO not found, log all spec names to help debug
	if coo == "" {
		log.Printf("[GET-ITEM-DEBUG] Item %s: COO NOT FOUND. All ItemSpecifics: %v", itemID, allSpecNames)
	}

	// Extract US international shipping cost
	foundUSShipping := false
	for _, intlOption := range xmlResp.Item.ShippingDetails.InternationalShippingServiceOption {
		for _, location := range intlOption.ShipToLocation {
			if location == "US" || location == "United States" || location == "Worldwide" {
				shippingCost = intlOption.ShippingServiceCost.Value
				shippingCurrency = intlOption.ShippingServiceCost.CurrencyID
				foundUSShipping = true
				log.Printf("[GET-ITEM-DEBUG] Item %s: US shipping = %s %s", itemID, shippingCost, shippingCurrency)
				break
			}
		}
		if foundUSShipping {
			break
		}
	}

	// Fallback to domestic shipping if no international option found
	if !foundUSShipping && len(xmlResp.Item.ShippingDetails.ShippingServiceOptions) > 0 {
		shippingCost = xmlResp.Item.ShippingDetails.ShippingServiceOptions[0].ShippingServiceCost.Value
		shippingCurrency = xmlResp.Item.ShippingDetails.ShippingServiceOptions[0].ShippingServiceCost.CurrencyID
		log.Printf("[GET-ITEM-DEBUG] Item %s: No US shipping, using domestic = %s %s", itemID, shippingCost, shippingCurrency)
	}

	// Extract all image URLs and convert to full-size (s-l1600)
	images = make([]string, 0, len(xmlResp.Item.PictureDetails.PictureURL))
	for _, imageURL := range xmlResp.Item.PictureDetails.PictureURL {
		// Convert eBay image URLs to full-size (1600px max dimension)
		// eBay URLs typically have size parameters like s-l64, s-l140, s-l225, s-l500
		fullSizeURL := strings.ReplaceAll(imageURL, "/s-l64.", "/s-l1600.")
		fullSizeURL = strings.ReplaceAll(fullSizeURL, "/s-l140.", "/s-l1600.")
		fullSizeURL = strings.ReplaceAll(fullSizeURL, "/s-l225.", "/s-l1600.")
		fullSizeURL = strings.ReplaceAll(fullSizeURL, "/s-l500.", "/s-l1600.")
		images = append(images, fullSizeURL)
	}
	log.Printf("[GET-ITEM-DEBUG] Item %s: Found %d image(s)", itemID, len(images))

	return brand, shippingCost, shippingCurrency, coo, images, nil
}

// GetMyeBaySelling fetches active listings using the Trading API (XML)
func (c *Client) GetMyeBaySelling(ctx context.Context, pageNumber, entriesPerPage int) ([]TradingItem, int, error) {
	if !c.IsAuthenticated() {
		return nil, 0, fmt.Errorf("client not authenticated")
	}

	// Ensure token is fresh
	src := c.oauthConfig.TokenSource(ctx, c.token)
	token, err := src.Token()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get valid token: %w", err)
	}
	c.token = token

	// Build XML request
	xmlRequest := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<GetMyeBaySellingRequest xmlns="urn:ebay:apis:eBLBaseComponents">
  <DetailLevel>ReturnAll</DetailLevel>
  <ActiveList>
    <Include>true</Include>
    <Pagination>
      <EntriesPerPage>%d</EntriesPerPage>
      <PageNumber>%d</PageNumber>
    </Pagination>
  </ActiveList>
</GetMyeBaySellingRequest>`, entriesPerPage, pageNumber)

	log.Printf("[TRADING-API-DEBUG] Request: page=%d, entries=%d", pageNumber, entriesPerPage)
	log.Printf("[TRADING-API-DEBUG] URL: %s", c.tradingAPIURL)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.tradingAPIURL, strings.NewReader(xmlRequest))
	if err != nil {
		return nil, 0, err
	}

	// Set headers for Trading API
	// Trading API uses IAF (Identity Assertion Framework) which requires X-EBAY-API-IAF-TOKEN header
	req.Header.Set("X-EBAY-API-COMPATIBILITY-LEVEL", "967")
	req.Header.Set("X-EBAY-API-CALL-NAME", "GetMyeBaySelling")
	req.Header.Set("X-EBAY-API-SITEID", "15") // Australia
	req.Header.Set("X-EBAY-API-IAF-TOKEN", token.AccessToken)
	req.Header.Set("Content-Type", "text/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[TRADING-API-ERROR] Request failed: %v", err)
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	log.Printf("[TRADING-API-DEBUG] Response status: %d", resp.StatusCode)
	log.Printf("[TRADING-API-DEBUG] Response body (first 1000 chars): %s", string(body)[:min(1000, len(body))])

	// Parse XML response
	var xmlResp GetMyeBaySellingResponse
	if err := xml.Unmarshal(body, &xmlResp); err != nil {
		log.Printf("[TRADING-API-ERROR] Failed to parse XML: %v", err)
		log.Printf("[TRADING-API-ERROR] Full response: %s", string(body))
		return nil, 0, fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Check for API errors
	if xmlResp.Ack != "Success" && xmlResp.Ack != "Warning" {
		if len(xmlResp.Errors) > 0 {
			errMsg := fmt.Sprintf("eBay API error %s: %s", xmlResp.Errors[0].ErrorCode, xmlResp.Errors[0].LongMessage)
			log.Printf("[TRADING-API-ERROR] %s", errMsg)
			return nil, 0, fmt.Errorf(errMsg)
		}
		return nil, 0, fmt.Errorf("API returned Ack=%s", xmlResp.Ack)
	}

	// Convert XML items to TradingItem structs
	items := make([]TradingItem, 0, len(xmlResp.ActiveList.ItemArray.Items))
	for i, xmlItem := range xmlResp.ActiveList.ItemArray.Items {
		// Extract image URL (prefer GalleryURL, fallback to first PictureURL)
		imageURL := xmlItem.PictureDetails.GalleryURL
		if imageURL == "" && len(xmlItem.PictureDetails.PictureURL) > 0 {
			imageURL = xmlItem.PictureDetails.PictureURL[0]
		}

		// Extract Brand from ItemSpecifics
		brand := ""
		if i == 0 {
			log.Printf("[BRAND-DEBUG] Item %s ItemSpecifics count: %d", xmlItem.ItemID, len(xmlItem.ItemSpecifics.NameValueList))
			for _, spec := range xmlItem.ItemSpecifics.NameValueList {
				log.Printf("[BRAND-DEBUG]   %s = %s", spec.Name, spec.Value)
			}
		}
		for _, spec := range xmlItem.ItemSpecifics.NameValueList {
			if spec.Name == "Brand" {
				brand = spec.Value
				if i == 0 {
					log.Printf("[BRAND-DEBUG] Found Brand: %s", brand)
				}
				break
			}
		}

		// Extract shipping cost - prefer international shipping to United States
		shippingCost := ""
		shippingCurrency := ""

		// Debug log shipping details for first item
		if i == 0 {
			log.Printf("[SHIPPING-DEBUG] Item %s (%s):", xmlItem.ItemID, xmlItem.Title)
			log.Printf("[SHIPPING-DEBUG]   Domestic options: %d", len(xmlItem.ShippingDetails.ShippingServiceOptions))
			log.Printf("[SHIPPING-DEBUG]   International options: %d", len(xmlItem.ShippingDetails.InternationalShippingServiceOption))
			for idx, intl := range xmlItem.ShippingDetails.InternationalShippingServiceOption {
				log.Printf("[SHIPPING-DEBUG]     Intl[%d] cost=%s %s, locations=%v",
					idx, intl.ShippingServiceCost.Value, intl.ShippingServiceCost.CurrencyID, intl.ShipToLocation)
			}
		}

		// First, try to find international shipping to US
		foundUSShipping := false
		for _, intlOption := range xmlItem.ShippingDetails.InternationalShippingServiceOption {
			// Check if this service ships to US (could be "US", "United States", or "Worldwide")
			for _, location := range intlOption.ShipToLocation {
				if location == "US" || location == "United States" || location == "Worldwide" {
					shippingCost = intlOption.ShippingServiceCost.Value
					shippingCurrency = intlOption.ShippingServiceCost.CurrencyID
					foundUSShipping = true
					if i == 0 {
						log.Printf("[SHIPPING-DEBUG] Found US shipping: %s %s", shippingCost, shippingCurrency)
					}
					break
				}
			}
			if foundUSShipping {
				break
			}
		}

		// Fallback to domestic shipping if no international option found
		if !foundUSShipping && len(xmlItem.ShippingDetails.ShippingServiceOptions) > 0 {
			shippingCost = xmlItem.ShippingDetails.ShippingServiceOptions[0].ShippingServiceCost.Value
			shippingCurrency = xmlItem.ShippingDetails.ShippingServiceOptions[0].ShippingServiceCost.CurrencyID
			if i == 0 {
				log.Printf("[SHIPPING-DEBUG] No US shipping found, using domestic: %s %s", shippingCost, shippingCurrency)
			}
		}

		item := TradingItem{
			ItemID:          xmlItem.ItemID,
			SKU:             xmlItem.SKU,
			Title:           xmlItem.Title,
			Price:           xmlItem.SellingStatus.CurrentPrice.Value,
			Currency:        xmlItem.SellingStatus.CurrentPrice.CurrencyID,
			Quantity:        xmlItem.Quantity,
			QuantitySold:    xmlItem.SellingStatus.QuantitySold,
			ImageURL:        imageURL,
			Brand:           brand,
			ShippingCost:    shippingCost,
			ShippingCurrency: shippingCurrency,
		}
		items = append(items, item)
	}

	totalEntries := xmlResp.ActiveList.PaginationResult.TotalNumberOfEntries
	log.Printf("[TRADING-API-DEBUG] Successfully parsed %d items (total: %d)", len(items), totalEntries)

	return items, totalEntries, nil
}
