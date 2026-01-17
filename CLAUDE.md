# eBay Postage Helper - Developer Context

## Quick Start
```bash
./restart-prod.sh  # Rebuild and restart production server
```
Server runs at http://localhost:8080 with ngrok tunnel for eBay OAuth callbacks.

## Behaviour Preferences
- Do not ask permission to rebuild and restart the app after updates
- Use `restart-prod.sh` script for server restarts (handles kill, build, verify)
- Do not ask permission for file edits - all edits to files in this repository are pre-approved
- Do not ask permission for file reads - all reads from files in this repository are pre-approved

---

## Architecture Overview

Single-instance Go web app that helps manage eBay listings with focus on:
1. **US Postage calculation** - Based on item value, weight, brand COO, and tariffs
2. **Listing validation** - Brand/COO compliance checking for US tariff requirements
3. **Data sync** - Export/import between eBay accounts via SQLite

### Key Components

```
cmd/server/
├── main.go           # Entry point, routes, embedded static files (//go:embed web/*)
└── web/              # Frontend (vanilla JS, no framework)
    ├── index.html    # Single page with tabs (Listings, Calculator, Sync, Reference)
    └── app.js        # All frontend logic, state management, API calls

internal/
├── calculator/       # Postage calculation logic (AusPost rates, tariffs, Zonos fees)
├── database/         # SQLite operations, session store, schema
├── ebay/             # eBay API client (Trading API XML, Browse API REST, OAuth)
├── handlers/         # HTTP handlers with concurrent enrichment
└── sync/             # Export/import service between accounts
```

### Data Flow

1. **Listings Load**: Frontend → `/api/offers` → Trading API `GetMyeBaySelling` (concurrent pages)
2. **Enrichment**: Frontend batches → `/api/offers/enriched` → Trading API `GetItem` + Browse API fallback (30 concurrent goroutines)
3. **Calculations**: Frontend → `/api/calculate/batch` → Server-side postage calculation

---

## Critical Business Logic

### Country of Origin (COO) - CRITICAL for US Tariffs
- **Primary source**: Trading API `GetItem` → `ItemSpecifics` (various field names)
- **Fallback**: Browse API `getItem` → `localizedAspects` (catches eBay-enriched data)
- Field names to check: `Country of Origin`, `Country/Region of Manufacture`, `Materials sourced from`

### Brand Validation
- Brand must be populated (red `[MISSING]` if empty)
- Brand should appear in listing title (orange `[NOT IN TITLE]` if mismatch)

### Postage Calculation
Location: `internal/calculator/calculator.go`
- AusPost Zone 3 rates (USA/Canada)
- Tariff rates by COO country
- Zonos duty fees
- Extra cover for high-value items

---

## eBay API Details

### Authentication
- OAuth 2.0 with session-based token storage (`internal/database/session_store.go`)
- Tokens stored in encrypted cookies, NOT in database
- Scopes: `api_scope`, `sell.inventory`, `sell.account`, `sell.fulfillment`, `commerce.identity`

### API Endpoints Used
| API | Base URL | Auth | Purpose |
|-----|----------|------|---------|
| Trading API | `api.ebay.com/ws/api.dll` | X-EBAY-API-IAF-TOKEN | GetMyeBaySelling, GetItem |
| Browse API | `api.ebay.com/buy/browse/v1` | Bearer token | COO fallback via localizedAspects |
| Commerce API | `apiz.ebay.com` | Bearer token | User identity |

### Rate Limits
- Production: ~5000 calls/day
- Current concurrency: 5 workers for listings, 30 for enrichment

---

## Performance Optimisations

1. **Listing fetch**: 5 concurrent goroutines fetch pages in parallel
2. **Enrichment**: 30 concurrent goroutines, frontend sends 2 batches of 60 simultaneously
3. **Caching**: 8-hour TTL on listings cache, enrichment cache persists until refresh

---

## Environment Variables

Required:
- `EBAY_CLIENT_ID` - eBay Developer App ID
- `EBAY_CLIENT_SECRET` - eBay Developer App Secret

Optional:
- `EBAY_REDIRECT_URI` - OAuth callback (default: localhost, set for ngrok)
- `EBAY_VERIFICATION_TOKEN` - For marketplace deletion endpoint
- `EBAY_PUBLIC_ENDPOINT` - Public URL for deletion notifications
- `EBAY_SESSION_SECRET` - Cookie encryption key (generate with `openssl rand -base64 32`)

---

## Common Maintenance Tasks

### Adding new COO field names
Edit `internal/ebay/client.go` → `GetItem()` and `GetItemFromBrowseAPI()` - search for `specNameLower` conditions.

### Adjusting concurrency
- Listings: `internal/handlers/handlers.go` → `maxWorkers = 5`
- Enrichment backend: `internal/handlers/handlers.go` → `maxConcurrent = 30`
- Enrichment frontend: `cmd/server/web/app.js` → `batchSize` and `parallelBatches`

### Adding new brands/COO mappings
Edit `internal/calculator/calculator.go` → `brandCountryMap`

### Modifying tariff rates
Edit `internal/database/schema.sql` → `tariff_countries` table defaults
