import 'package:flutter/material.dart';
import '../services/ebay_api_service.dart';
import '../services/credentials_service.dart';
import '../services/data_service.dart';
import '../services/calculator_service.dart';
import '../models/calculation_result.dart';
import '../widgets/listing_row.dart';
import '../widgets/calculator_dialog.dart';

/// Main listings screen showing all eBay listings with postage comparison
class ListingsScreen extends StatefulWidget {
  final VoidCallback onLogout;

  const ListingsScreen({super.key, required this.onLogout});

  @override
  State<ListingsScreen> createState() => _ListingsScreenState();
}

class _ListingsScreenState extends State<ListingsScreen> {
  final EbayApiService _ebayApi = EbayApiService.instance;
  final DataService _dataService = DataService.instance;
  final CalculatorService _calculator = CalculatorService();

  List<ListingWithCalculation> _listings = [];
  bool _isLoading = true;
  String? _errorMessage;
  int _totalListings = 0;
  int _currentPage = 0;
  static const int _pageSize = 25;

  // Filters
  String _searchQuery = '';
  bool _showOnlyMismatched = false;

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  Future<void> _loadData() async {
    setState(() {
      _isLoading = true;
      _errorMessage = null;
    });

    try {
      // Load lookup data if not already loaded
      if (!_dataService.isLoaded) {
        await _dataService.loadAll();
      }

      // Fetch listings from eBay
      await _fetchListings();
    } catch (e) {
      setState(() {
        _errorMessage = e.toString();
      });
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  Future<void> _fetchListings() async {
    try {
      final response = await _ebayApi.getOffers(
        limit: _pageSize,
        offset: _currentPage * _pageSize,
      );

      final listings = <ListingWithCalculation>[];

      for (final offer in response.offers) {
        // Get inventory item details for brand/product info
        InventoryItem? inventoryItem;
        try {
          final itemsResponse = await _ebayApi.getInventoryItems(limit: 1, offset: 0);
          inventoryItem = itemsResponse.inventoryItems
              .where((item) => item.sku == offer.sku)
              .firstOrNull;
        } catch (_) {
          // Inventory item lookup is optional
        }

        // Calculate expected postage
        final brandName = inventoryItem?.product?.brand;
        final itemValue = offer.pricingSummary?.numericValue ?? 0.0;

        // Default to Medium weight band if unknown
        const weightBand = 'Medium';

        ShippingCalculationResult? calculation;
        if (itemValue > 0) {
          calculation = _calculator.calculateUSAShipping(
            itemValueAUD: itemValue,
            weightBand: weightBand,
            brandName: brandName,
          );
        }

        // Get current US shipping override if any
        double? currentUSShipping;
        final overrides = offer.listingPolicies?.shippingCostOverrides;
        if (overrides != null) {
          final usOverride = overrides.where(
            (o) => o.shippingServiceType == 'INTERNATIONAL',
          ).firstOrNull;
          currentUSShipping = usOverride?.shippingCost?.numericValue;
        }

        listings.add(ListingWithCalculation(
          offer: offer,
          inventoryItem: inventoryItem,
          calculation: calculation,
          currentUSShipping: currentUSShipping,
        ));
      }

      setState(() {
        _listings = listings;
        _totalListings = response.total;
      });
    } catch (e) {
      setState(() {
        _errorMessage = 'Failed to fetch listings: $e';
      });
    }
  }

  List<ListingWithCalculation> get _filteredListings {
    var filtered = _listings;

    if (_searchQuery.isNotEmpty) {
      final query = _searchQuery.toLowerCase();
      filtered = filtered.where((l) {
        final title = l.inventoryItem?.product?.title?.toLowerCase() ?? '';
        final sku = l.offer.sku.toLowerCase();
        return title.contains(query) || sku.contains(query);
      }).toList();
    }

    if (_showOnlyMismatched) {
      filtered = filtered.where((l) => l.hasMismatch).toList();
    }

    return filtered;
  }

  void _showCalculatorDialog() {
    showDialog(
      context: context,
      builder: (context) => const CalculatorDialog(),
    );
  }

  void _showSettingsDialog() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Settings'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.logout),
              title: const Text('Change API Credentials'),
              subtitle: const Text('Update or reconfigure eBay API settings'),
              onTap: () {
                Navigator.pop(context);
                widget.onLogout();
              },
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Close'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('eBay Postage Helper'),
        actions: [
          IconButton(
            icon: const Icon(Icons.calculate),
            tooltip: 'Postage Calculator',
            onPressed: _showCalculatorDialog,
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: 'Refresh',
            onPressed: _isLoading ? null : _loadData,
          ),
          IconButton(
            icon: const Icon(Icons.settings),
            tooltip: 'Settings',
            onPressed: _showSettingsDialog,
          ),
        ],
      ),
      body: Column(
        children: [
          // Filters bar
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surface,
              border: Border(
                bottom: BorderSide(
                  color: Theme.of(context).dividerColor,
                ),
              ),
            ),
            child: Row(
              children: [
                // Search
                Expanded(
                  child: TextField(
                    decoration: const InputDecoration(
                      hintText: 'Search by title or SKU...',
                      prefixIcon: Icon(Icons.search),
                      border: OutlineInputBorder(),
                      contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                    ),
                    onChanged: (value) {
                      setState(() {
                        _searchQuery = value;
                      });
                    },
                  ),
                ),
                const SizedBox(width: 16),

                // Filter toggle
                FilterChip(
                  label: const Text('Show mismatched only'),
                  selected: _showOnlyMismatched,
                  onSelected: (selected) {
                    setState(() {
                      _showOnlyMismatched = selected;
                    });
                  },
                ),
                const SizedBox(width: 16),

                // Stats
                Text(
                  '${_filteredListings.length} of $_totalListings listings',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
          ),

          // Content
          Expanded(
            child: _buildContent(),
          ),

          // Pagination
          if (!_isLoading && _listings.isNotEmpty)
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                border: Border(
                  top: BorderSide(
                    color: Theme.of(context).dividerColor,
                  ),
                ),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  IconButton(
                    icon: const Icon(Icons.chevron_left),
                    onPressed: _currentPage > 0
                        ? () {
                            setState(() {
                              _currentPage--;
                            });
                            _fetchListings();
                          }
                        : null,
                  ),
                  Text('Page ${_currentPage + 1} of ${(_totalListings / _pageSize).ceil()}'),
                  IconButton(
                    icon: const Icon(Icons.chevron_right),
                    onPressed: (_currentPage + 1) * _pageSize < _totalListings
                        ? () {
                            setState(() {
                              _currentPage++;
                            });
                            _fetchListings();
                          }
                        : null,
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildContent() {
    if (_isLoading) {
      return const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            CircularProgressIndicator(),
            SizedBox(height: 16),
            Text('Loading listings...'),
          ],
        ),
      );
    }

    if (_errorMessage != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: Colors.red[300]),
            const SizedBox(height: 16),
            Text(
              'Error loading listings',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            Text(
              _errorMessage!,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: Colors.grey[600],
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            FilledButton.icon(
              onPressed: _loadData,
              icon: const Icon(Icons.refresh),
              label: const Text('Retry'),
            ),
          ],
        ),
      );
    }

    if (_filteredListings.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.inventory_2_outlined, size: 48, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              _showOnlyMismatched
                  ? 'No mismatched listings found'
                  : 'No listings found',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            Text(
              _showOnlyMismatched
                  ? 'All listings have matching postage values'
                  : 'Try adjusting your search or adding listings in eBay Seller Hub',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: Colors.grey[600],
              ),
            ),
          ],
        ),
      );
    }

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: SingleChildScrollView(
        child: DataTable(
          columnSpacing: 24,
          headingRowColor: WidgetStateProperty.all(
            Theme.of(context).colorScheme.surfaceContainerHighest,
          ),
          columns: const [
            DataColumn(label: Text('Image')),
            DataColumn(label: Text('Title / SKU')),
            DataColumn(label: Text('Brand')),
            DataColumn(label: Text('Price (AUD)')),
            DataColumn(label: Text('Weight')),
            DataColumn(label: Text('Current US Postage')),
            DataColumn(label: Text('Calculated')),
            DataColumn(label: Text('Status')),
            DataColumn(label: Text('Actions')),
          ],
          rows: _filteredListings.map((listing) {
            return ListingRow(
              listing: listing,
              onRecalculate: () => _showRecalculateDialog(listing),
            ).buildRow(context);
          }).toList(),
        ),
      ),
    );
  }

  void _showRecalculateDialog(ListingWithCalculation listing) {
    showDialog(
      context: context,
      builder: (context) => RecalculateDialog(
        listing: listing,
        calculator: _calculator,
        dataService: _dataService,
      ),
    );
  }
}

/// Combined listing data with calculation
class ListingWithCalculation {
  final Offer offer;
  final InventoryItem? inventoryItem;
  final ShippingCalculationResult? calculation;
  final double? currentUSShipping;

  ListingWithCalculation({
    required this.offer,
    this.inventoryItem,
    this.calculation,
    this.currentUSShipping,
  });

  String get title => inventoryItem?.product?.title ?? offer.sku;
  String? get brand => inventoryItem?.product?.brand;
  String? get imageUrl => inventoryItem?.product?.imageUrls?.firstOrNull;
  double get price => offer.pricingSummary?.numericValue ?? 0.0;

  bool get hasMismatch {
    if (calculation == null || currentUSShipping == null) return false;
    return (currentUSShipping! - calculation!.totalShipping).abs() > 1.0;
  }

  double get mismatchAmount {
    if (calculation == null || currentUSShipping == null) return 0.0;
    return currentUSShipping! - calculation!.totalShipping;
  }
}

/// Dialog for recalculating postage with different parameters
class RecalculateDialog extends StatefulWidget {
  final ListingWithCalculation listing;
  final CalculatorService calculator;
  final DataService dataService;

  const RecalculateDialog({
    super.key,
    required this.listing,
    required this.calculator,
    required this.dataService,
  });

  @override
  State<RecalculateDialog> createState() => _RecalculateDialogState();
}

class _RecalculateDialogState extends State<RecalculateDialog> {
  late String _selectedWeightBand;
  late String? _selectedBrand;
  late bool _includeExtraCover;
  ShippingCalculationResult? _result;

  @override
  void initState() {
    super.initState();
    _selectedWeightBand = 'Medium';
    _selectedBrand = widget.listing.brand;
    _includeExtraCover = widget.listing.price >= 250;
    _calculate();
  }

  void _calculate() {
    final result = widget.calculator.calculateUSAShipping(
      itemValueAUD: widget.listing.price,
      weightBand: _selectedWeightBand,
      brandName: _selectedBrand,
      includeExtraCover: _includeExtraCover,
    );
    setState(() {
      _result = result;
    });
  }

  @override
  Widget build(BuildContext context) {
    final weightBands = ['XSmall', 'Small', 'Medium', 'Large', 'XLarge'];
    final brands = widget.dataService.getAvailableBrands();

    return AlertDialog(
      title: Text('Calculate Postage: ${widget.listing.title}'),
      content: SizedBox(
        width: 400,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Item Value: \$${widget.listing.price.toStringAsFixed(2)} AUD'),
            const SizedBox(height: 16),

            // Weight band selector
            DropdownButtonFormField<String>(
              value: _selectedWeightBand,
              decoration: const InputDecoration(
                labelText: 'Weight Band',
                border: OutlineInputBorder(),
              ),
              items: weightBands.map((band) {
                return DropdownMenuItem(value: band, child: Text(band));
              }).toList(),
              onChanged: (value) {
                setState(() {
                  _selectedWeightBand = value!;
                });
                _calculate();
              },
            ),
            const SizedBox(height: 16),

            // Brand selector
            DropdownButtonFormField<String?>(
              value: brands.contains(_selectedBrand) ? _selectedBrand : null,
              decoration: const InputDecoration(
                labelText: 'Brand (for Country of Origin)',
                border: OutlineInputBorder(),
              ),
              items: [
                const DropdownMenuItem(value: null, child: Text('Unknown (Default: China)')),
                ...brands.map((brand) {
                  return DropdownMenuItem(value: brand, child: Text(brand));
                }),
              ],
              onChanged: (value) {
                setState(() {
                  _selectedBrand = value;
                });
                _calculate();
              },
            ),
            const SizedBox(height: 16),

            // Extra cover checkbox
            CheckboxListTile(
              title: const Text('Include Extra Cover (Insurance)'),
              subtitle: Text(
                widget.listing.price >= 250
                    ? 'Recommended for items >= \$250'
                    : 'Optional for items < \$250',
              ),
              value: _includeExtraCover,
              onChanged: (value) {
                setState(() {
                  _includeExtraCover = value!;
                });
                _calculate();
              },
            ),
            const SizedBox(height: 16),

            // Results
            if (_result != null) ...[
              const Divider(),
              const SizedBox(height: 8),
              Text(
                'Calculation Breakdown',
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              _buildResultRow('AusPost Shipping', _result!.breakdown.ausPostShipping),
              _buildResultRow('Extra Cover', _result!.breakdown.extraCover),
              _buildResultRow('Tariff Duties (${_result!.inputs.countryOfOrigin})', _result!.breakdown.tariffDuties),
              _buildResultRow('Zonos Fees', _result!.breakdown.zonosFees),
              const Divider(),
              _buildResultRow('TOTAL', _result!.totalShipping, isBold: true),

              if (_result!.warnings.extraCoverRecommended)
                Container(
                  margin: const EdgeInsets.only(top: 8),
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: Colors.orange[50],
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.warning_amber, color: Colors.orange[700], size: 16),
                      const SizedBox(width: 8),
                      Text(
                        'Extra Cover recommended for items >= \$250',
                        style: TextStyle(color: Colors.orange[900], fontSize: 12),
                      ),
                    ],
                  ),
                ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Close'),
        ),
      ],
    );
  }

  Widget _buildResultRow(String label, double value, {bool isBold = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: isBold ? const TextStyle(fontWeight: FontWeight.bold) : null,
          ),
          Text(
            '\$${value.toStringAsFixed(2)}',
            style: isBold ? const TextStyle(fontWeight: FontWeight.bold) : null,
          ),
        ],
      ),
    );
  }
}
