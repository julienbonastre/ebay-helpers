package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/julienbonastre/ebay-helpers/internal/ebay"
	"github.com/julienbonastre/ebay-helpers/internal/handlers"
)

//go:embed web/*
var webFS embed.FS

func main() {
	// Command line flags
	port := flag.String("port", "8080", "Server port")
	sandbox := flag.Bool("sandbox", true, "Use eBay sandbox environment")
	flag.Parse()

	// Get eBay credentials from environment
	clientID := os.Getenv("EBAY_CLIENT_ID")
	clientSecret := os.Getenv("EBAY_CLIENT_SECRET")
	redirectURI := os.Getenv("EBAY_REDIRECT_URI")

	if redirectURI == "" {
		redirectURI = "http://localhost:" + *port + "/api/oauth/callback"
	}

	// Create eBay client
	ebayClient := ebay.NewClient(ebay.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Sandbox:      *sandbox,
	})

	// Create handlers
	h := handlers.NewHandler(ebayClient)

	// Set up routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/health", h.HealthCheck)
	mux.HandleFunc("/api/auth/url", h.GetAuthURL)
	mux.HandleFunc("/api/auth/status", h.GetAuthStatus)
	mux.HandleFunc("/api/oauth/callback", h.OAuthCallback)
	mux.HandleFunc("/api/inventory", h.GetInventoryItems)
	mux.HandleFunc("/api/offers", h.GetOffers)
	mux.HandleFunc("/api/policies", h.GetFulfillmentPolicies)
	mux.HandleFunc("/api/calculate", h.CalculateShipping)
	mux.HandleFunc("/api/brands", h.GetBrands)
	mux.HandleFunc("/api/weight-bands", h.GetWeightBands)
	mux.HandleFunc("/api/tariff-countries", h.GetTariffCountries)
	mux.HandleFunc("/api/update-shipping", h.UpdateOfferShipping)

	// Serve embedded static files
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	// Start server
	addr := ":" + *port
	log.Printf("Starting eBay Postage Helper on http://localhost%s", addr)
	log.Printf("Sandbox mode: %v", *sandbox)

	if clientID == "" {
		log.Println("WARNING: EBAY_CLIENT_ID not set - eBay API calls will fail")
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
