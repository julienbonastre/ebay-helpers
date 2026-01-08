package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/julienbonastre/ebay-helpers/internal/database"
	"github.com/julienbonastre/ebay-helpers/internal/handlers"
)

//go:embed web/*
var webFS embed.FS

func main() {
	// Command line flags
	port := flag.String("port", "8080", "Server port")
	dbPath := flag.String("db", "ebay-helpers.db", "SQLite database path")
	sandbox := flag.Bool("sandbox", true, "Use eBay sandbox environment")
	storeName := flag.String("store", "", "Store name for account tracking (e.g., 'la_troverie')")
	flag.Parse()

	// Get eBay credentials from environment
	clientID := os.Getenv("EBAY_CLIENT_ID")
	clientSecret := os.Getenv("EBAY_CLIENT_SECRET")
	redirectURI := os.Getenv("EBAY_REDIRECT_URI")
	marketplaceID := os.Getenv("EBAY_MARKETPLACE_ID")

	if redirectURI == "" {
		redirectURI = "http://localhost:" + *port + "/api/oauth/callback"
	}
	if marketplaceID == "" {
		marketplaceID = "EBAY_AU"
	}

	// Determine account identifier
	environment := "sandbox"
	if !*sandbox {
		environment = "production"
	}

	accountKey := *storeName + "_" + environment + "_" + marketplaceID
	displayName := *storeName
	if environment == "production" {
		displayName += " Production"
	} else {
		displayName += " Sandbox"
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

	// Get or create account record
	var currentAccount *database.Account
	if *storeName != "" {
		currentAccount, err = db.GetOrCreateAccount(accountKey, displayName, environment, marketplaceID)
		if err != nil {
			log.Fatalf("Failed to get/create account: %v", err)
		}
		log.Printf("Account: %s (%s)", currentAccount.DisplayName, currentAccount.AccountKey)
	}

	// Create eBay client
	ebayClient := ebay.NewClient(ebay.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Sandbox:      *sandbox,
	})

	// Create handlers
	h := handlers.NewHandler(db, ebayClient, currentAccount)

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

	// eBay API
	mux.HandleFunc("/api/inventory", h.GetInventoryItems)
	mux.HandleFunc("/api/offers", h.GetOffers)
	mux.HandleFunc("/api/policies", h.GetFulfillmentPolicies)
	mux.HandleFunc("/api/update-shipping", h.UpdateOfferShipping)

	// Sync operations
	mux.HandleFunc("/api/sync/export", h.SyncExport)         // Export current eBay → DB
	mux.HandleFunc("/api/sync/import", h.SyncImport)         // Import DB → current eBay
	mux.HandleFunc("/api/sync/history", h.GetSyncHistory)

	// Calculator
	mux.HandleFunc("/api/calculate", h.CalculateShipping)
	mux.HandleFunc("/api/brands", h.GetBrands)
	mux.HandleFunc("/api/weight-bands", h.GetWeightBands)
	mux.HandleFunc("/api/tariff-countries", h.GetTariffCountries)

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
	if currentAccount != nil {
		log.Printf("Account: %s", currentAccount.DisplayName)
		log.Printf("Environment: %s", currentAccount.Environment)
		log.Printf("Marketplace: %s", currentAccount.MarketplaceID)
	} else {
		log.Println("Account: Not specified (use -store flag)")
	}
	log.Println("")
	log.Println("Workflow:")
	log.Println("  1. Export: Run with production credentials → Click 'Export' to save to DB")
	log.Println("  2. Import: Restart with sandbox credentials → Click 'Import' to restore")
	log.Println("")
	log.Println("Example:")
	log.Println("  # Export from production")
	log.Println("  EBAY_CLIENT_ID=xxx EBAY_CLIENT_SECRET=yyy \\")
	log.Println("    ./ebay-postage-helper -sandbox=false -store=la_troverie")
	log.Println("")
	log.Println("  # Import to sandbox")
	log.Println("  EBAY_CLIENT_ID=sandbox_xxx EBAY_CLIENT_SECRET=sandbox_yyy \\")
	log.Println("    ./ebay-postage-helper -sandbox=true -store=la_troverie")
	log.Println("==================================================================")
	log.Println("")

	if clientID == "" {
		log.Println("WARNING: EBAY_CLIENT_ID not set - eBay API calls will fail")
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
