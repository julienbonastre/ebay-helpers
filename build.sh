#!/bin/bash
set -e

# Build the eBay Postage Helper
# Outputs to /tmp to avoid accidental git commits
echo "Building ebay-postage-helper..."
go build -o /tmp/ebay-postage-helper ./cmd/server

echo "✓ Build complete: /tmp/ebay-postage-helper"
echo ""
echo "Usage Examples:"
echo "  # Sandbox mode with account tracking"
echo "  EBAY_CLIENT_ID=sandbox_xxx EBAY_CLIENT_SECRET=sandbox_yyy \\"
echo "    /tmp/ebay-postage-helper -sandbox=true -store=la_troverie"
echo ""
echo "  # Production mode"
echo "  EBAY_CLIENT_ID=prod_xxx EBAY_CLIENT_SECRET=prod_yyy \\"
echo "    /tmp/ebay-postage-helper -sandbox=false -store=la_troverie"
echo ""
echo "Options:"
echo "  -port=8080              Server port (default: 8080)"
echo "  -db=ebay-helpers.db     Database path (default: ebay-helpers.db)"
echo "  -sandbox=true           Use eBay sandbox environment (default: true)"
echo "  -store=STORENAME        Store name for account tracking (required for sync features)"
echo ""
echo "Environment Variables:"
echo "  EBAY_CLIENT_ID              Your eBay Developer App ID (required)"
echo "  EBAY_CLIENT_SECRET          Your eBay Developer App Secret (required)"
echo "  EBAY_REDIRECT_URI           OAuth callback URL (optional, auto-generated)"
echo "  EBAY_MARKETPLACE_ID         Marketplace (default: EBAY_AU)"
echo "  EBAY_VERIFICATION_TOKEN     Token for marketplace deletion endpoint verification"
echo "  EBAY_PUBLIC_ENDPOINT        Public URL for marketplace deletion notifications"
echo ""
echo "For Production API Activation:"
echo "  Set EBAY_VERIFICATION_TOKEN and EBAY_PUBLIC_ENDPOINT before activating"
echo "  your production credentials in eBay Developer Portal."
echo "  Endpoint: /api/marketplace-account-deletion"
