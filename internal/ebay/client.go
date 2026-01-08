package ebay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	// Sandbox URLs
	SandboxAuthURL     = "https://auth.sandbox.ebay.com/oauth2/authorize"
	SandboxTokenURL    = "https://api.sandbox.ebay.com/identity/v1/oauth2/token"
	SandboxAPIBaseURL  = "https://api.sandbox.ebay.com"

	// Production URLs
	ProductionAuthURL    = "https://auth.ebay.com/oauth2/authorize"
	ProductionTokenURL   = "https://api.ebay.com/identity/v1/oauth2/token"
	ProductionAPIBaseURL = "https://api.ebay.com"
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
	config      Config
	httpClient  *http.Client
	oauthConfig *oauth2.Config
	token       *oauth2.Token
	baseURL     string
}

// NewClient creates a new eBay API client
func NewClient(cfg Config) *Client {
	var authURL, tokenURL, baseURL string
	if cfg.Sandbox {
		authURL = SandboxAuthURL
		tokenURL = SandboxTokenURL
		baseURL = SandboxAPIBaseURL
	} else {
		authURL = ProductionAuthURL
		tokenURL = ProductionTokenURL
		baseURL = ProductionAPIBaseURL
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
		config:      cfg,
		oauthConfig: oauthConfig,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		baseURL:     baseURL,
	}
}

// GetAuthURL returns the OAuth authorization URL
func (c *Client) GetAuthURL(state string) string {
	return c.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCode exchanges an auth code for tokens
func (c *Client) ExchangeCode(ctx context.Context, code string) error {
	token, err := c.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}
	c.token = token
	return nil
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

// doRequest makes an authenticated API request
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

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result InventoryItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GetOffers retrieves offers for a SKU or all offers
func (c *Client) GetOffers(ctx context.Context, sku string, limit, offset int) (*OffersResponse, error) {
	path := fmt.Sprintf("/sell/inventory/v1/offer?limit=%d&offset=%d", limit, offset)
	if sku != "" {
		path += "&sku=" + url.QueryEscape(sku)
	}

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result OffersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
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
