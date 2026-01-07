import 'package:flutter/material.dart';
import '../services/data_service.dart';
import '../services/calculator_service.dart';
import '../models/calculation_result.dart';

/// Standalone calculator dialog for quick postage calculations
class CalculatorDialog extends StatefulWidget {
  const CalculatorDialog({super.key});

  @override
  State<CalculatorDialog> createState() => _CalculatorDialogState();
}

class _CalculatorDialogState extends State<CalculatorDialog> {
  final DataService _dataService = DataService.instance;
  final CalculatorService _calculator = CalculatorService();
  final _itemValueController = TextEditingController(text: '100');

  String _selectedWeightBand = 'Medium';
  String? _selectedBrand;
  bool _includeExtraCover = false;
  int _discountBand = 0;
  ShippingCalculationResult? _result;

  @override
  void initState() {
    super.initState();
    _calculate();
  }

  @override
  void dispose() {
    _itemValueController.dispose();
    super.dispose();
  }

  void _calculate() {
    final itemValue = double.tryParse(_itemValueController.text) ?? 0;
    if (itemValue <= 0) {
      setState(() {
        _result = null;
      });
      return;
    }

    final result = _calculator.calculateUSAShipping(
      itemValueAUD: itemValue,
      weightBand: _selectedWeightBand,
      brandName: _selectedBrand,
      includeExtraCover: _includeExtraCover,
      discountBand: _discountBand,
    );
    setState(() {
      _result = result;
    });
  }

  @override
  Widget build(BuildContext context) {
    final weightBands = ['XSmall', 'Small', 'Medium', 'Large', 'XLarge'];
    final weightLabels = {
      'XSmall': 'XSmall (< 250g)',
      'Small': 'Small (250-500g)',
      'Medium': 'Medium (500g-1kg)',
      'Large': 'Large (1-1.5kg)',
      'XLarge': 'XLarge (1.5-2kg)',
    };
    final brands = _dataService.getAvailableBrands();

    return Dialog(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 500),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Header
              Row(
                children: [
                  const Icon(Icons.calculate),
                  const SizedBox(width: 8),
                  Text(
                    'US Postage Calculator',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const Spacer(),
                  IconButton(
                    icon: const Icon(Icons.close),
                    onPressed: () => Navigator.pop(context),
                  ),
                ],
              ),
              const Divider(),
              const SizedBox(height: 16),

              // Item Value
              TextField(
                controller: _itemValueController,
                decoration: const InputDecoration(
                  labelText: 'Item Value (AUD)',
                  prefixText: '\$ ',
                  border: OutlineInputBorder(),
                ),
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                onChanged: (_) => _calculate(),
              ),
              const SizedBox(height: 16),

              // Weight band
              DropdownButtonFormField<String>(
                value: _selectedWeightBand,
                decoration: const InputDecoration(
                  labelText: 'Weight Band',
                  border: OutlineInputBorder(),
                ),
                items: weightBands.map((band) {
                  return DropdownMenuItem(
                    value: band,
                    child: Text(weightLabels[band] ?? band),
                  );
                }).toList(),
                onChanged: (value) {
                  setState(() {
                    _selectedWeightBand = value!;
                  });
                  _calculate();
                },
              ),
              const SizedBox(height: 16),

              // Brand
              DropdownButtonFormField<String?>(
                value: brands.contains(_selectedBrand) ? _selectedBrand : null,
                decoration: const InputDecoration(
                  labelText: 'Brand (for Country of Origin)',
                  border: OutlineInputBorder(),
                ),
                items: [
                  const DropdownMenuItem(
                    value: null,
                    child: Text('Unknown (Default: China)'),
                  ),
                  ...brands.map((brand) {
                    final coo = _dataService.getCountryOfOrigin(brand);
                    return DropdownMenuItem(
                      value: brand,
                      child: Text('$brand ($coo)'),
                    );
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

              // Extra cover
              SwitchListTile(
                title: const Text('Include Extra Cover (Insurance)'),
                subtitle: _result?.warnings.extraCoverRecommended == true
                    ? Text(
                        'Recommended for items >= \$250',
                        style: TextStyle(color: Colors.orange[700]),
                      )
                    : const Text('Optional insurance coverage'),
                value: _includeExtraCover,
                onChanged: (value) {
                  setState(() {
                    _includeExtraCover = value;
                  });
                  _calculate();
                },
              ),
              const SizedBox(height: 16),

              // Discount band
              DropdownButtonFormField<int>(
                value: _discountBand,
                decoration: const InputDecoration(
                  labelText: 'Discount Band (MyPost Business)',
                  border: OutlineInputBorder(),
                ),
                items: const [
                  DropdownMenuItem(value: 0, child: Text('Band 0 (No discount)')),
                  DropdownMenuItem(value: 1, child: Text('Band 1 (5% off)')),
                  DropdownMenuItem(value: 2, child: Text('Band 2 (15% off)')),
                  DropdownMenuItem(value: 3, child: Text('Band 3 (20% off)')),
                  DropdownMenuItem(value: 4, child: Text('Band 4 (25% off)')),
                  DropdownMenuItem(value: 5, child: Text('Band 5 (30% off)')),
                ],
                onChanged: (value) {
                  setState(() {
                    _discountBand = value!;
                  });
                  _calculate();
                },
              ),
              const SizedBox(height: 24),

              // Results
              if (_result != null) ...[
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: Theme.of(context).colorScheme.surfaceContainerHighest,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Calculation Breakdown',
                        style: Theme.of(context).textTheme.titleSmall,
                      ),
                      const SizedBox(height: 12),
                      _buildRow('Country of Origin', _result!.inputs.countryOfOrigin),
                      _buildRow('Tariff Rate', '${(_result!.inputs.tariffRate * 100).toStringAsFixed(0)}%'),
                      const Divider(height: 24),
                      _buildMoneyRow('AusPost Shipping', _result!.breakdown.ausPostShipping),
                      _buildMoneyRow('Extra Cover', _result!.breakdown.extraCover),
                      _buildMoneyRow('Tariff Duties', _result!.breakdown.tariffDuties),
                      _buildMoneyRow('Zonos Fees', _result!.breakdown.zonosFees),
                      const Divider(height: 24),
                      _buildMoneyRow(
                        'TOTAL US SHIPPING',
                        _result!.totalShipping,
                        isBold: true,
                        isLarge: true,
                      ),
                    ],
                  ),
                ),
              ],

              if (_result?.warnings.extraCoverRecommended == true && !_includeExtraCover)
                Container(
                  margin: const EdgeInsets.only(top: 12),
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Colors.orange[50],
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: Colors.orange[200]!),
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.warning_amber, color: Colors.orange[700]),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          'Extra Cover is recommended for items valued at \$250 or more',
                          style: TextStyle(color: Colors.orange[900]),
                        ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(color: Colors.grey)),
          Text(value),
        ],
      ),
    );
  }

  Widget _buildMoneyRow(String label, double value, {bool isBold = false, bool isLarge = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: TextStyle(
              fontWeight: isBold ? FontWeight.bold : null,
              fontSize: isLarge ? 16 : null,
            ),
          ),
          Text(
            '\$${value.toStringAsFixed(2)}',
            style: TextStyle(
              fontWeight: isBold ? FontWeight.bold : null,
              fontSize: isLarge ? 18 : null,
              color: isLarge ? Theme.of(context).primaryColor : null,
            ),
          ),
        ],
      ),
    );
  }
}
