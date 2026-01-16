# eBay Production API Activation

This document explains how to configure the marketplace account deletion notification endpoint required for activating your eBay production API credentials.

## Overview

eBay requires all production applications to implement a marketplace account deletion notification endpoint. This endpoint allows eBay to notify your application when users request their account data to be deleted, in compliance with privacy regulations (GDPR, CCPA, etc.).

Documentation: https://developer.ebay.com/develop/guides-v2/marketplace-user-account-deletion

## How It Works

1. **Endpoint Validation**: eBay sends a GET request with a `challenge_code` parameter to verify your endpoint is working
2. **Challenge Response**: Your app must compute SHA-256 hash of: `challengeCode + verificationToken + endpoint` and return it
3. **Deletion Notifications**: eBay sends POST requests when users request account deletion
4. **Storage & Compliance**: Your app stores these notifications for audit purposes

## Configuration Steps

### 1. Generate a Verification Token

Create a random verification token (this will be used to verify incoming notifications):

```bash
# Generate a random token (example)
openssl rand -hex 32
# Output: e.g., "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6"
```

### 2. Set Up Public Endpoint

You need a publicly accessible HTTPS URL. For local development, use ngrok:

```bash
# Install ngrok if you haven't already
# Start ngrok tunnel
ngrok http 8080

# You'll get a URL like: https://abc123.ngrok.io
```

Your endpoint URL will be: `https://abc123.ngrok.io/api/marketplace-account-deletion`

### 3. Start the Server

```bash
# Set environment variables
export EBAY_VERIFICATION_TOKEN="your-verification-token-here"
export EBAY_PUBLIC_ENDPOINT="https://abc123.ngrok.io/api/marketplace-account-deletion"

# For production
export EBAY_CLIENT_ID="your-production-client-id"
export EBAY_CLIENT_SECRET="your-production-client-secret"

# Start server
/tmp/ebay-postage-helper -sandbox=false -store=la_troverie
```

### 4. Configure in eBay Developer Portal

1. Go to https://developer.ebay.com/my/keys
2. Select your production keyset
3. Under "Event Notification Delivery Method", select "Subscription"
4. Enter your endpoint URL: `https://abc123.ngrok.io/api/marketplace-account-deletion`
5. Enter your verification token (same one you set in EBAY_VERIFICATION_TOKEN)
6. Click "Send test notification" to verify it works
7. Save your configuration

## Testing the Endpoint

### Test Challenge Validation (GET)

```bash
curl "http://localhost:8080/api/marketplace-account-deletion?challenge_code=test123"
```

Expected response:
```json
{
  "challengeResponse": "some-hex-hash-here"
}
```

### Test Deletion Notification (POST)

```bash
curl -X POST http://localhost:8080/api/marketplace-account-deletion \
  -H 'Content-Type: application/json' \
  -d '{
    "metadata": {
      "topic": "MARKETPLACE_ACCOUNT_DELETION",
      "schemaVersion": "1.0"
    },
    "notification": {
      "notificationId": "test-123",
      "eventDate": "2026-01-09T00:00:00Z",
      "data": {
        "username": "testuser",
        "userId": "U123",
        "eiasToken": "token123"
      }
    }
  }'
```

Expected: HTTP 200 OK

### View Stored Notifications

```bash
curl http://localhost:8080/api/deletion-notifications
```

## Important Notes

### Data Handling

This application uses **memory-only OAuth token storage** - tokens are never persisted to disk or database. Tokens are lost on server restart, requiring re-authentication via OAuth each time.

Since no persistent user credentials are stored, deletion notifications are logged for eBay compliance but no actual data deletion occurs. The notification is automatically marked as "processed" upon receipt.

**Data stored:**
- ✅ Inventory/policy data (account owner's business data, not end-user data)
- ✅ Deletion notifications (audit trail for compliance)
- ❌ OAuth tokens (memory-only, not persisted)

**Trade-off:**
- ✅ No persistent user data = no deletion obligations
- ✅ Maximum security (credentials never hit disk)
- ⚠️ Must re-authenticate after every server restart

### Security

- Use HTTPS in production (ngrok provides this automatically)
- Keep your verification token secret
- The verification token is used to validate incoming challenges from eBay
- Store all notifications in the database for audit trail compliance

### Production Deployment

For a production deployment (not using ngrok):

1. Deploy your application on a server with a public domain
2. Set up SSL/TLS certificate (Let's Encrypt is free)
3. Configure your firewall to allow HTTPS traffic
4. Use a proper secret management system for tokens
5. Monitor the `/api/deletion-notifications` endpoint for compliance

Example production configuration:

```bash
export EBAY_VERIFICATION_TOKEN="$(cat /etc/ebay-helpers/verification-token)"
export EBAY_PUBLIC_ENDPOINT="https://ebay-helper.yourdomain.com/api/marketplace-account-deletion"
export EBAY_CLIENT_ID="your-production-client-id"
export EBAY_CLIENT_SECRET="$(cat /etc/ebay-helpers/client-secret)"

/opt/ebay-helpers/bin/ebay-postage-helper \
  -sandbox=false \
  -store=la_troverie \
  -port=8080 \
  -db=/var/lib/ebay-helpers/data.db
```

## Troubleshooting

**Challenge validation fails:**
- Verify your verification token matches what's in the eBay Developer Portal
- Check the endpoint URL is exactly correct (including https://)
- Look at server logs for the computed hash

**Notifications not being received:**
- Check your ngrok/server is running and accessible
- Verify the endpoint returns 200 OK for POST requests
- Check firewall settings

**Database errors:**
- Ensure the database file is writable
- Check disk space
- Review server logs for SQL errors

## API Endpoints

- `GET /api/marketplace-account-deletion?challenge_code=XXX` - Challenge validation
- `POST /api/marketplace-account-deletion` - Receive deletion notifications
- `GET /api/deletion-notifications` - View stored notifications (admin)

## Compliance Notes

Per eBay requirements, applications must:
1. ✅ Provide a publicly accessible HTTPS endpoint
2. ✅ Respond to challenge validation requests
3. ✅ Accept and acknowledge deletion notifications with 200 OK
4. ✅ Store notifications for audit purposes
5. ✅ Delete user data within required timeframe

**Compliance Status:**
- **OAuth Tokens:** Memory-only storage (not persisted) = No deletion required
- **Inventory/Policy Data:** Business data belonging to account owner, not end-user = No deletion required
- **Deletion Notifications:** Stored for compliance audit trail

This application satisfies all applicable requirements for eBay marketplace account deletion compliance. The endpoint is functional and required for production API activation, even though no persistent user credentials are stored.
