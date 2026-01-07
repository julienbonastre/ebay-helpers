# eBay Postage Helper

A Go utility for managing eBay listing postage/shipping costs, specifically designed for Australian sellers shipping to the United States with the current IEEPA tariff regime.

## Features

- **Shipping Calculator**: Calculate total US shipping costs including:
  - Australia Post international rates (Zone 3 - USA/Canada)
  - US IEEPA tariffs based on country of origin
  - Zonos processing fees
  - Extra Cover (insurance) options

- **Brand → Country Mapping**: Automatic lookup of country of origin by brand name

- **eBay Integration**:
  - OAuth2 authentication with eBay API
  - View active listings with current postage settings
  - Compare current vs calculated postage amounts
  - Bulk update shipping cost overrides (Phase 2)

## Quick Start

### 1. Build

```bash
go build -o ebay-postage-helper ./cmd/server
```

### 2. Configure eBay Credentials

Set environment variables for your eBay Developer API credentials:

```bash
export EBAY_CLIENT_ID="your-client-id"
export EBAY_CLIENT_SECRET="your-client-secret"
export EBAY_REDIRECT_URI="http://localhost:8080/api/oauth/callback"
```

### 3. Run

```bash
# Sandbox mode (default)
./ebay-postage-helper

# Production mode
./ebay-postage-helper -sandbox=false

# Custom port
./ebay-postage-helper -port=3000
```

### 4. Open Browser

Navigate to http://localhost:8080

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |
| `/api/auth/url` | GET | Get eBay OAuth URL |
| `/api/auth/status` | GET | Check auth status |
| `/api/oauth/callback` | GET | OAuth callback handler |
| `/api/calculate` | POST | Calculate shipping costs |
| `/api/brands` | GET | List available brands |
| `/api/weight-bands` | GET | List weight bands |
| `/api/tariff-countries` | GET | List tariff rates by country |
| `/api/inventory` | GET | Get eBay inventory items |
| `/api/offers` | GET | Get eBay offers/listings |
| `/api/policies` | GET | Get fulfillment policies |
| `/api/update-shipping` | POST | Update shipping overrides |

## Calculation Logic

### Total US Shipping Formula

```
Total = AusPost Shipping + Extra Cover (optional) + Tariff Duties + Zonos Fees
```

Where:
- **AusPost Shipping** = Base Rate × (1 + 2% handling) × (1 - discount band)
- **Extra Cover** = ((ItemValue - $100) / 100) × $4 × (1 - discount)
- **Tariff Duties** = Item Value × Tariff Rate
- **Zonos Fees** = (Tariff Duties × 10%) + $1.69

### Current Tariff Rates (US IEEPA)

| Country | Rate |
|---------|------|
| India | 50% |
| Mexico | 25% |
| China | 20% |
| Vietnam | 20% |
| Indonesia | 19% |
| Malaysia | 19% |
| Japan | 15% |
| Australia | 10% |
| United States | 0% |

### Weight Bands (Zone 3 - USA/Canada)

| Band | Weight | Base Price (AUD) |
|------|--------|------------------|
| XSmall | < 250g | $22.30 |
| Small | 250 - 500g | $29.00 |
| Medium | 500g - 1kg | $42.20 |
| Large | 1 - 1.5kg | $55.55 |
| XLarge | 1.5 - 2kg | $68.85 |

## Project Structure

```
ebay-helpers/
├── cmd/server/
│   ├── main.go              # Entry point
│   └── web/                  # Embedded frontend
│       ├── index.html
│       └── app.js
├── internal/
│   ├── calculator/          # Shipping cost calculations
│   │   ├── calculator.go
│   │   └── data.go
│   ├── ebay/                # eBay API client
│   │   └── client.go
│   └── handlers/            # HTTP handlers
│       └── handlers.go
├── src/data/                # Reference JSON data
├── go.mod
├── go.sum
└── TariffAndPostalCalculator.xlsx  # Original spreadsheet
```

## Roadmap

### Phase 1 (Current)
- [x] Shipping cost calculator
- [x] eBay OAuth integration
- [x] View listings with postage comparison
- [x] Reference data display

### Phase 2 (Next)
- [ ] Bulk edit shipping overrides
- [ ] Item weight/brand detection from titles
- [ ] Fulfillment policy management
- [ ] Export/import configuration

### Future
- [ ] Live AusPost rate fetching
- [ ] Zonos API integration
- [ ] Multiple marketplace support
- [ ] Price history tracking

## Data Sources

- **Postal Rates**: Australia Post International Parcel rates
- **Tariff Rates**: US IEEPA Reciprocal Tariffs (2025)
- **Zonos Fees**: Zonos customs clearance processing
