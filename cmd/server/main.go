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
	flag.Parse()

	// Initialize database
	log.Printf("Opening database: %s", *dbPath)
	// Ensure directory exists
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
	log.Println("Database initialized successfully")

	// Create handlers with database
	h := handlers.NewHandler(db, "http://localhost:"+*port)

	// Set up routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/health", h.HealthCheck)

	// Account management
	mux.HandleFunc("/api/accounts", h.HandleAccounts)           // GET=list, POST=create
	mux.HandleFunc("/api/accounts/active", h.SetActiveAccount)  // POST

	// OAuth (per-account)
	mux.HandleFunc("/api/auth/url", h.GetAuthURL)
	mux.HandleFunc("/api/auth/status", h.GetAuthStatus)
	mux.HandleFunc("/api/oauth/callback", h.OAuthCallback)

	// eBay API (uses active account)
	mux.HandleFunc("/api/inventory", h.GetInventoryItems)
	mux.HandleFunc("/api/offers", h.GetOffers)
	mux.HandleFunc("/api/policies", h.GetFulfillmentPolicies)
	mux.HandleFunc("/api/update-shipping", h.UpdateOfferShipping)

	// Sync operations
	mux.HandleFunc("/api/sync/export", h.ExportToDatabase)
	mux.HandleFunc("/api/sync/import", h.ImportFromDatabase)
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
	log.Printf("Starting eBay Postage Helper on http://localhost%s", addr)
	log.Printf("Database: %s", *dbPath)
	log.Println("")
	log.Println("==================================================================")
	log.Println("Multi-Account Setup:")
	log.Println("  1. Visit http://localhost" + addr)
	log.Println("  2. Go to 'Accounts' tab to register your eBay stores")
	log.Println("  3. Add accounts like 'La Troverie Production', 'La Troverie Sandbox', etc.")
	log.Println("  4. Connect each account separately via OAuth")
	log.Println("  5. Use 'Sync' tab to export/import between accounts")
	log.Println("==================================================================")
	log.Println("")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
