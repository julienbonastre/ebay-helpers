import 'package:flutter/material.dart';
import '../screens/listings_screen.dart';

/// Widget helper for building listing table rows
class ListingRow {
  final ListingWithCalculation listing;
  final VoidCallback onRecalculate;

  ListingRow({
    required this.listing,
    required this.onRecalculate,
  });

  DataRow buildRow(BuildContext context) {
    final hasMismatch = listing.hasMismatch;
    final mismatchAmount = listing.mismatchAmount;

    return DataRow(
      color: hasMismatch
          ? WidgetStateProperty.all(Colors.orange[50])
          : null,
      cells: [
        // Image
        DataCell(
          listing.imageUrl != null
              ? ClipRRect(
                  borderRadius: BorderRadius.circular(4),
                  child: Image.network(
                    listing.imageUrl!,
                    width: 50,
                    height: 50,
                    fit: BoxFit.cover,
                    errorBuilder: (_, __, ___) => _buildPlaceholder(),
                  ),
                )
              : _buildPlaceholder(),
        ),

        // Title / SKU
        DataCell(
          SizedBox(
            width: 250,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Text(
                  listing.title,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontWeight: FontWeight.w500),
                ),
                Text(
                  'SKU: ${listing.offer.sku}',
                  style: TextStyle(
                    fontSize: 12,
                    color: Colors.grey[600],
                  ),
                ),
              ],
            ),
          ),
        ),

        // Brand
        DataCell(
          Text(
            listing.brand ?? 'Unknown',
            style: TextStyle(
              color: listing.brand == null ? Colors.grey[400] : null,
              fontStyle: listing.brand == null ? FontStyle.italic : null,
            ),
          ),
        ),

        // Price
        DataCell(
          Text(
            '\$${listing.price.toStringAsFixed(2)}',
            style: const TextStyle(fontWeight: FontWeight.w500),
          ),
        ),

        // Weight (placeholder - would need to be set manually or from item specifics)
        DataCell(
          DropdownButton<String>(
            value: 'Medium',
            underline: const SizedBox(),
            isDense: true,
            items: const [
              DropdownMenuItem(value: 'XSmall', child: Text('XSmall')),
              DropdownMenuItem(value: 'Small', child: Text('Small')),
              DropdownMenuItem(value: 'Medium', child: Text('Medium')),
              DropdownMenuItem(value: 'Large', child: Text('Large')),
              DropdownMenuItem(value: 'XLarge', child: Text('XLarge')),
            ],
            onChanged: (_) {
              // TODO: Implement weight change
            },
          ),
        ),

        // Current US Postage
        DataCell(
          listing.currentUSShipping != null
              ? Text(
                  '\$${listing.currentUSShipping!.toStringAsFixed(2)}',
                  style: TextStyle(
                    color: hasMismatch ? Colors.orange[800] : null,
                  ),
                )
              : Text(
                  'Not set',
                  style: TextStyle(
                    color: Colors.grey[400],
                    fontStyle: FontStyle.italic,
                  ),
                ),
        ),

        // Calculated
        DataCell(
          listing.calculation != null
              ? Text(
                  '\$${listing.calculation!.totalShipping.toStringAsFixed(2)}',
                  style: const TextStyle(fontWeight: FontWeight.w500),
                )
              : Text(
                  'N/A',
                  style: TextStyle(color: Colors.grey[400]),
                ),
        ),

        // Status
        DataCell(
          _buildStatusChip(hasMismatch, mismatchAmount),
        ),

        // Actions
        DataCell(
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              IconButton(
                icon: const Icon(Icons.calculate, size: 20),
                tooltip: 'Recalculate',
                onPressed: onRecalculate,
              ),
              if (hasMismatch)
                IconButton(
                  icon: Icon(Icons.sync, size: 20, color: Colors.orange[700]),
                  tooltip: 'Update postage',
                  onPressed: () {
                    // TODO: Implement update
                  },
                ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildPlaceholder() {
    return Container(
      width: 50,
      height: 50,
      decoration: BoxDecoration(
        color: Colors.grey[200],
        borderRadius: BorderRadius.circular(4),
      ),
      child: Icon(
        Icons.image_not_supported_outlined,
        color: Colors.grey[400],
        size: 24,
      ),
    );
  }

  Widget _buildStatusChip(bool hasMismatch, double mismatchAmount) {
    if (!hasMismatch) {
      return Chip(
        label: const Text('OK'),
        backgroundColor: Colors.green[100],
        labelStyle: TextStyle(
          color: Colors.green[800],
          fontSize: 12,
        ),
        visualDensity: VisualDensity.compact,
        padding: EdgeInsets.zero,
      );
    }

    final isUndercharging = mismatchAmount < 0;
    return Chip(
      label: Text(
        isUndercharging
            ? '-\$${mismatchAmount.abs().toStringAsFixed(2)}'
            : '+\$${mismatchAmount.toStringAsFixed(2)}',
      ),
      backgroundColor: isUndercharging ? Colors.red[100] : Colors.orange[100],
      labelStyle: TextStyle(
        color: isUndercharging ? Colors.red[800] : Colors.orange[800],
        fontSize: 12,
        fontWeight: FontWeight.w500,
      ),
      visualDensity: VisualDensity.compact,
      padding: EdgeInsets.zero,
    );
  }
}
