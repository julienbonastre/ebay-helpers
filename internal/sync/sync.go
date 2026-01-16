package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/julienbonastre/ebay-helpers/internal/database"
	"github.com/julienbonastre/ebay-helpers/internal/ebay"
)

// Service handles sync operations between eBay accounts and local database
type Service struct {
	db *database.DB
}

// NewService creates a new sync service
func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// ExportFromEbay exports all data from eBay account to local database
func (s *Service) ExportFromEbay(ctx context.Context, client *ebay.Client, accountID int64, marketplaceID string) error {
	syncHistory := &database.SyncHistory{
		AccountID: accountID,
		SyncType:  "export",
		Status:    "running",
		StartedAt: time.Now(),
	}
	if err := s.db.CreateSyncHistory(syncHistory); err != nil {
		return fmt.Errorf("failed to create sync history: %w", err)
	}

	totalItems := 0
	var lastErr error

	// Export fulfillment policies
	log.Printf("Exporting fulfillment policies...")
	if count, err := s.exportFulfillmentPolicies(ctx, client, accountID, marketplaceID); err != nil {
		log.Printf("Error exporting fulfillment policies: %v", err)
		lastErr = err
	} else {
		totalItems += count
		log.Printf("Exported %d fulfillment policies", count)
	}

	// Export payment policies
	log.Printf("Exporting payment policies...")
	if count, err := s.exportPaymentPolicies(ctx, client, accountID, marketplaceID); err != nil {
		log.Printf("Error exporting payment policies: %v", err)
		lastErr = err
	} else {
		totalItems += count
		log.Printf("Exported %d payment policies", count)
	}

	// Export return policies
	log.Printf("Exporting return policies...")
	if count, err := s.exportReturnPolicies(ctx, client, accountID, marketplaceID); err != nil {
		log.Printf("Error exporting return policies: %v", err)
		lastErr = err
	} else {
		totalItems += count
		log.Printf("Exported %d return policies", count)
	}

	// Export inventory items
	log.Printf("Exporting inventory items...")
	if count, err := s.exportInventoryItems(ctx, client, accountID); err != nil {
		log.Printf("Error exporting inventory: %v", err)
		lastErr = err
	} else {
		totalItems += count
		log.Printf("Exported %d inventory items", count)
	}

	// Export offers
	log.Printf("Exporting offers...")
	if count, err := s.exportOffers(ctx, client, accountID); err != nil {
		log.Printf("Error exporting offers: %v", err)
		lastErr = err
	} else {
		totalItems += count
		log.Printf("Exported %d offers", count)
	}

	// Update sync history
	now := time.Now()
	syncHistory.CompletedAt = &now
	syncHistory.ItemsSynced = totalItems
	if lastErr != nil {
		syncHistory.Status = "partial"
		syncHistory.ErrorMessage = lastErr.Error()
	} else {
		syncHistory.Status = "success"
	}
	if err := s.db.UpdateSyncHistory(syncHistory); err != nil {
		return fmt.Errorf("failed to update sync history: %w", err)
	}

	log.Printf("Export complete: %d total items", totalItems)
	return lastErr
}

func (s *Service) exportFulfillmentPolicies(ctx context.Context, client *ebay.Client, accountID int64, marketplaceID string) (int, error) {
	resp, err := client.GetFulfillmentPolicies(ctx, marketplaceID)
	if err != nil {
		return 0, err
	}

	for _, policy := range resp.FulfillmentPolicies {
		data, err := json.Marshal(policy)
		if err != nil {
			log.Printf("Failed to marshal policy %s: %v", policy.FulfillmentPolicyID, err)
			continue
		}

		_, err = s.db.Exec(`
			INSERT OR REPLACE INTO fulfillment_policies (account_id, policy_id, name, marketplace_id, data, updated_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, accountID, policy.FulfillmentPolicyID, policy.Name, policy.MarketplaceID, string(data))
		if err != nil {
			log.Printf("Failed to save policy %s: %v", policy.FulfillmentPolicyID, err)
		}
	}

	return len(resp.FulfillmentPolicies), nil
}

func (s *Service) exportPaymentPolicies(ctx context.Context, client *ebay.Client, accountID int64, marketplaceID string) (int, error) {
	resp, err := client.GetPaymentPolicies(ctx, marketplaceID)
	if err != nil {
		return 0, err
	}

	for _, policy := range resp.PaymentPolicies {
		data, err := json.Marshal(policy)
		if err != nil {
			log.Printf("Failed to marshal payment policy %s: %v", policy.PaymentPolicyID, err)
			continue
		}

		_, err = s.db.Exec(`
			INSERT OR REPLACE INTO payment_policies (account_id, policy_id, name, marketplace_id, data, updated_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, accountID, policy.PaymentPolicyID, policy.Name, policy.MarketplaceID, string(data))
		if err != nil {
			log.Printf("Failed to save payment policy %s: %v", policy.PaymentPolicyID, err)
		}
	}

	return len(resp.PaymentPolicies), nil
}

func (s *Service) exportReturnPolicies(ctx context.Context, client *ebay.Client, accountID int64, marketplaceID string) (int, error) {
	resp, err := client.GetReturnPolicies(ctx, marketplaceID)
	if err != nil {
		return 0, err
	}

	for _, policy := range resp.ReturnPolicies {
		data, err := json.Marshal(policy)
		if err != nil {
			log.Printf("Failed to marshal return policy %s: %v", policy.ReturnPolicyID, err)
			continue
		}

		_, err = s.db.Exec(`
			INSERT OR REPLACE INTO return_policies (account_id, policy_id, name, marketplace_id, data, updated_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, accountID, policy.ReturnPolicyID, policy.Name, policy.MarketplaceID, string(data))
		if err != nil {
			log.Printf("Failed to save return policy %s: %v", policy.ReturnPolicyID, err)
		}
	}

	return len(resp.ReturnPolicies), nil
}

func (s *Service) exportInventoryItems(ctx context.Context, client *ebay.Client, accountID int64) (int, error) {
	const batchSize = 100
	offset := 0
	totalCount := 0

	for {
		resp, err := client.GetInventoryItems(ctx, batchSize, offset)
		if err != nil {
			return totalCount, err
		}

		if len(resp.InventoryItems) == 0 {
			break
		}

		for _, item := range resp.InventoryItems {
			data, err := json.Marshal(item)
			if err != nil {
				log.Printf("Failed to marshal item %s: %v", item.SKU, err)
				continue
			}

			title := ""
			brand := ""
			if item.Product != nil {
				title = item.Product.Title
				brand = item.Product.Brand
			}

			_, err = s.db.Exec(`
				INSERT OR REPLACE INTO inventory_items (account_id, sku, title, brand, condition, data, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			`, accountID, item.SKU, title, brand, item.Condition, string(data))
			if err != nil {
				log.Printf("Failed to save item %s: %v", item.SKU, err)
			}
		}

		totalCount += len(resp.InventoryItems)
		offset += batchSize

		// If we got fewer than batch size, we're done
		if len(resp.InventoryItems) < batchSize {
			break
		}
	}

	return totalCount, nil
}

func (s *Service) exportOffers(ctx context.Context, client *ebay.Client, accountID int64) (int, error) {
	const batchSize = 100
	offset := 0
	totalCount := 0

	for {
		resp, err := client.GetOffers(ctx, "", batchSize, offset)
		if err != nil {
			return totalCount, err
		}

		if len(resp.Offers) == 0 {
			break
		}

		for _, offer := range resp.Offers {
			data, err := json.Marshal(offer)
			if err != nil {
				log.Printf("Failed to marshal offer %s: %v", offer.OfferID, err)
				continue
			}

			listingID := ""
			if offer.Listing != nil {
				listingID = offer.Listing.ListingID
			}

			_, err = s.db.Exec(`
				INSERT OR REPLACE INTO offers (account_id, offer_id, sku, marketplace_id, listing_id, status, data, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			`, accountID, offer.OfferID, offer.SKU, offer.MarketplaceID, listingID, offer.Status, string(data))
			if err != nil {
				log.Printf("Failed to save offer %s: %v", offer.OfferID, err)
			}
		}

		totalCount += len(resp.Offers)
		offset += batchSize

		if len(resp.Offers) < batchSize {
			break
		}
	}

	return totalCount, nil
}

// ImportToEbay reads from DB and creates items in target eBay account
// NOTE: This is a basic implementation. Full policy creation requires additional eBay API methods.
func (s *Service) ImportToEbay(ctx context.Context, client *ebay.Client, sourceAccountID, targetAccountID int64) error {
	syncHistory := &database.SyncHistory{
		AccountID: targetAccountID,
		SyncType:  "import",
		Status:    "running",
		StartedAt: time.Now(),
	}
	if err := s.db.CreateSyncHistory(syncHistory); err != nil {
		return fmt.Errorf("failed to create sync history: %w", err)
	}

	totalItems := 0
	var lastErr error

	// Import inventory items
	log.Printf("Importing inventory items...")
	if count, err := s.importInventoryItems(ctx, client, sourceAccountID); err != nil {
		log.Printf("Error importing inventory: %v", err)
		lastErr = err
	} else {
		totalItems += count
		log.Printf("Imported %d inventory items", count)
	}

	// Import offers (listings)
	// NOTE: Offers require policies to exist first. This is simplified for now.
	log.Printf("NOTE: Offer import requires policies to be manually configured in sandbox first")
	log.Printf("Skipping offer import for now - will be enhanced in future")

	// Update sync history
	now := time.Now()
	syncHistory.CompletedAt = &now
	syncHistory.ItemsSynced = totalItems
	if lastErr != nil {
		syncHistory.Status = "partial"
		syncHistory.ErrorMessage = lastErr.Error()
	} else {
		syncHistory.Status = "success"
	}
	if err := s.db.UpdateSyncHistory(syncHistory); err != nil {
		return fmt.Errorf("failed to update sync history: %w", err)
	}

	log.Printf("Import complete: %d total items", totalItems)
	return lastErr
}

func (s *Service) importInventoryItems(ctx context.Context, client *ebay.Client, sourceAccountID int64) (int, error) {
	// Read inventory items from database
	rows, err := s.db.Query(`
		SELECT sku, data
		FROM inventory_items
		WHERE account_id = ?
		ORDER BY created_at
	`, sourceAccountID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var sku, data string
		if err := rows.Scan(&sku, &data); err != nil {
			log.Printf("Failed to scan inventory item: %v", err)
			continue
		}

		// Parse JSON back to InventoryItem struct
		var item ebay.InventoryItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			log.Printf("Failed to unmarshal inventory item %s: %v", sku, err)
			continue
		}

		// TODO: Create inventory item in target eBay account
		// This requires implementing CreateInventoryItem method in ebay.Client
		log.Printf("TODO: Would import inventory item: %s - %s", sku, item.Product.Title)
		count++
	}

	return count, rows.Err()
}

