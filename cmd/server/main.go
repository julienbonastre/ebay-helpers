package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/sessions"
	"github.com/julienbonastre/ebay-helpers/internal/database"
	"github.com/julienbonastre/ebay-helpers/internal/ebay"
	"github.com/julienbonastre/ebay-helpers/internal/handlers"
)

//go:embed web/*
var webFS embed.FS

func main() {
	// Command line flags
	port := flag.String("port", "8080", "Server port")
	dbPath := flag.String("db", "ebay-helpers.db", "SQLite database path")
	sandbox := flag.Bool("sandbox", true, "Use eBay sandbox environment")
	storeName := flag.String("store", "", "(DEPRECATED) Account is now auto-created via OAuth")
	flag.Parse()

	// Get eBay credentials from environment
	clientID := os.Getenv("EBAY_CLIENT_ID")
	clientSecret := os.Getenv("EBAY_CLIENT_SECRET")
	redirectURI := os.Getenv("EBAY_REDIRECT_URI")
	marketplaceID := os.Getenv("EBAY_MARKETPLACE_ID")
	verificationToken := os.Getenv("EBAY_VERIFICATION_TOKEN")
	publicEndpoint := os.Getenv("EBAY_PUBLIC_ENDPOINT")
	sessionSecret := os.Getenv("EBAY_SESSION_SECRET")

	if redirectURI == "" {
		redirectURI = "http://localhost:" + *port + "/api/oauth/callback"
	}
	if marketplaceID == "" {
		marketplaceID = "EBAY_AU"
	}
	if verificationToken == "" {
		verificationToken = "changeme-verification-token"
		log.Println("WARNING: Using default EBAY_VERIFICATION_TOKEN. Set env var for production.")
	}
	if publicEndpoint == "" {
		publicEndpoint = "http://localhost:" + *port + "/api/marketplace-account-deletion"
		log.Println("INFO: Using default EBAY_PUBLIC_ENDPOINT. Set env var for production.")
	}
	if sessionSecret == "" {
		sessionSecret = "changeme-insecure-session-secret-please-set-env-var"
		log.Println("WARNING: Using default EBAY_SESSION_SECRET. Generate a secure random key for production!")
		log.Println("         Run: openssl rand -base64 32")
	}

	// Determine environment
	environment := "sandbox"
	if !*sandbox {
		environment = "production"
	}

	// Initialize database
	log.Printf("Opening database: %s", *dbPath)
	if dir := filepath.Dir(*dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create database directory: %v", err)
		}
	}

	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Seed initial data (brand-COO mappings, tariff rates)
	if err := db.SeedInitialData(); err != nil {
		log.Fatalf("Failed to seed initial data: %v", err)
	}

	// Account will be auto-created after OAuth authentication
	// No longer pre-creating accounts from -store flag
	if *storeName != "" {
		log.Printf("WARNING: -store flag is deprecated. Account will be auto-created from eBay username after OAuth.")
	}

	// Initialise database-backed session store (avoids 4KB cookie size limit)
	sessionStore := database.NewDBSessionStore(db, []byte(sessionSecret))
	sessionStore.SetOptions(&sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: true,
		Secure:   !*sandbox, // Only use Secure flag in production (requires HTTPS)
		SameSite: http.SameSiteLaxMode,
	})

	// Create eBay config for handlers
	ebayConfig := ebay.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Sandbox:      *sandbox,
	}

	// Create handlers with session store (no shared eBay client)
	h := handlers.NewHandler(db, ebayConfig, sessionStore, verificationToken, publicEndpoint, environment, marketplaceID)

	// Set up routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/health", h.HealthCheck)

	// Account info (read-only, shows current instance)
	mux.HandleFunc("/api/account/current", h.GetCurrentAccount)
	mux.HandleFunc("/api/accounts", h.GetAccounts) // List all accounts in DB

	// OAuth
	mux.HandleFunc("/api/auth/url", h.GetAuthURL)
	mux.HandleFunc("/api/auth/status", h.GetAuthStatus)
	mux.HandleFunc("/api/oauth/callback", h.OAuthCallback)
	mux.HandleFunc("/api/logout", h.Logout)

	// Marketplace Account Deletion (required for production API activation)
	mux.HandleFunc("/api/marketplace-account-deletion", h.MarketplaceAccountDeletion)
	mux.HandleFunc("/api/deletion-notifications", h.GetDeletionNotifications)

	// eBay API
	mux.HandleFunc("/api/inventory", h.GetInventoryItems)
	mux.HandleFunc("/api/offers", h.GetOffers)
	mux.HandleFunc("/api/offers/enriched", h.GetEnrichedData) // Progressive enrichment data
	mux.HandleFunc("/api/listings", h.GetListings)            // DB-backed listings with server-side sort/filter
	mux.HandleFunc("/api/policies", h.GetFulfillmentPolicies)
	mux.HandleFunc("/api/update-shipping", h.UpdateOfferShipping)

	// Sync operations
	mux.HandleFunc("/api/sync/export", h.SyncExport)         // Export current eBay → DB
	mux.HandleFunc("/api/sync/import", h.SyncImport)         // Import DB → current eBay
	mux.HandleFunc("/api/sync/history", h.GetSyncHistory)

	// Calculator
	mux.HandleFunc("/api/calculate", h.CalculateShipping)
	mux.HandleFunc("/api/calculate/batch", h.BatchCalculate) // Server-side batch calculation
	mux.HandleFunc("/api/brands", h.GetBrands)
	mux.HandleFunc("/api/weight-bands", h.GetWeightBands)
	mux.HandleFunc("/api/tariff-countries", h.GetTariffCountries)

	// Settings
	mux.HandleFunc("/api/settings", h.GetAllSettings)
	mux.HandleFunc("/api/settings/", h.UpdateSetting) // Handles /api/settings/:key

	// Serve embedded static files
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	// Start server
	addr := ":" + *port
	log.Println("")
	log.Println("==================================================================")
	log.Printf("eBay Postage Helper - http://localhost%s", addr)
	log.Println("==================================================================")
	log.Printf("Database: %s", *dbPath)
	log.Printf("Environment: %s", environment)
	log.Printf("Marketplace: %s", marketplaceID)
	log.Println("Account: Will be auto-created after OAuth authentication")
	log.Println("")
	log.Println("Marketplace Account Deletion Endpoint:")
	log.Printf("  %s", publicEndpoint)
	log.Println("  (Required for production API activation)")
	log.Println("")
	log.Println("Workflow:")
	log.Println("  1. Click 'Connect to eBay' to authenticate (account auto-created)")
	log.Println("  2. Export: Click 'Export' to save your eBay data to database")
	log.Println("  3. Import: Restart with different credentials → Click 'Import' to restore")
	log.Println("")
	log.Println("Example:")
	log.Println("  # Export from production")
	log.Println("  EBAY_CLIENT_ID=xxx EBAY_CLIENT_SECRET=yyy \\")
	log.Println("    ./ebay-postage-helper -sandbox=false")
	log.Println("")
	log.Println("  # Import to sandbox")
	log.Println("  EBAY_CLIENT_ID=sandbox_xxx EBAY_CLIENT_SECRET=sandbox_yyy \\")
	log.Println("    ./ebay-postage-helper -sandbox=true")
	log.Println("==================================================================")
	log.Println("")

	if clientID == "" {
		log.Println("WARNING: EBAY_CLIENT_ID not set - eBay API calls will fail")
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
