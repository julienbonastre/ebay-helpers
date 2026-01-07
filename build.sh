#!/bin/bash
set -e

# Build the eBay Postage Helper
echo "Building ebay-postage-helper..."
go build -o ebay-postage-helper ./cmd/server

echo "Done! Binary: ./ebay-postage-helper ($(ls -lh ebay-postage-helper | awk '{print $5}'))"
echo ""
echo "Usage:"
echo "  ./ebay-postage-helper              # Run on port 8080 (sandbox mode)"
echo "  ./ebay-postage-helper -port 3000   # Run on custom port"
echo "  ./ebay-postage-helper -sandbox=false  # Use production eBay API"
echo ""
echo "Environment variables (optional, for eBay API):"
echo "  EBAY_CLIENT_ID      - Your eBay Developer App ID"
echo "  EBAY_CLIENT_SECRET  - Your eBay Developer App Secret"
echo "  EBAY_REDIRECT_URI   - OAuth callback URL (default: http://localhost:8080/api/oauth/callback)"
