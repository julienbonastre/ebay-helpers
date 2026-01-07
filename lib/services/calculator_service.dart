import '../models/postal_data.dart';
import '../models/calculation_result.dart';
import 'data_service.dart';

/// Calculator service for US shipping costs including tariffs
///
/// Implements the calculation logic from the TariffAndPostalCalculator spreadsheet:
/// Total = AusPost Shipping + Extra Cover (optional) + Tariff Duties + Zonos Fees
class CalculatorService {
  final DataService _dataService;

  CalculatorService({DataService? dataService})
      : _dataService = dataService ?? DataService.instance;

  /// Calculate AusPost shipping cost
  /// Formula: Base × (1 + handling) × (1 - discount)
  double calculateAusPostShipping({
    required String zone,
    required String weightBand,
    int discountBand = 0,
  }) {
    final zoneData = _dataService.postalRates.zones[zone];
    if (zoneData == null) {
      throw ArgumentError('Unknown zone: $zone');
    }

    final weightData = zoneData.weightBands[weightBand];
    if (weightData == null) {
      throw ArgumentError('Unknown weight band: $weightBand');
    }

    final basePrice = weightData.basePrice;
    final handlingFee = zoneData.handlingFee;
    final discount = zoneData.discountBands[discountBand] ?? 0.0;

    final withHandling = basePrice * (1 + handlingFee);
    final finalPrice = withHandling * (1 - discount);

    return _roundTo2Decimals(finalPrice);
  }

  /// Calculate extra cover (insurance) cost
  /// Formula: (ItemValue - 100) / 100 × $4 × (1 - discount)
  double calculateExtraCover({
    required double itemValueAUD,
    int discountBand = 0,
  }) {
    final extraCover = _dataService.extraCover;

    if (itemValueAUD <= extraCover.thresholdAUD) {
      return 0.0;
    }

    final discount = extraCover.discountBands[discountBand] ?? 0.0;
    final coverUnits = (itemValueAUD - extraCover.thresholdAUD) / 100;
    final cost = coverUnits * extraCover.basePricePer100 * (1 - discount);

    return _roundTo2Decimals(cost);
  }

  /// Calculate US tariff duties
  /// Formula: ItemValue × TariffRate
  double calculateTariffDuties({
    required double itemValueAUD,
    required String countryOfOrigin,
  }) {
    final tariffRate = _dataService.getTariffRate(countryOfOrigin);
    return _roundTo2Decimals(itemValueAUD * tariffRate);
  }

  /// Calculate Zonos processing fees
  /// Formula: (TariffAmount × 10%) + $1.69
  double calculateZonosFees({required double tariffAmount}) {
    final tariffs = _dataService.tariffs;
    final percentageFee = tariffAmount * tariffs.zonosProcessingPercent;
    final total = percentageFee + tariffs.zonosFlatFeeAUD;
    return _roundTo2Decimals(total);
  }

  /// Check if extra cover warning should be shown
  bool shouldWarnExtraCover({
    required double itemValueAUD,
    required bool hasExtraCover,
  }) {
    return itemValueAUD >= _dataService.extraCover.warningThresholdAUD &&
           !hasExtraCover;
  }

  /// Calculate total shipping cost to USA including all fees
  ShippingCalculationResult calculateUSAShipping({
    required double itemValueAUD,
    required String weightBand,
    String? brandName,
    String? countryOfOriginOverride,
    bool includeExtraCover = false,
    int discountBand = 0,
  }) {
    const zone = '3-USA & Canada';

    // Determine country of origin
    final countryOfOrigin = countryOfOriginOverride ??
        _dataService.getCountryOfOrigin(brandName);
    final tariffRate = _dataService.getTariffRate(countryOfOrigin);

    // Calculate components
    final ausPostShipping = calculateAusPostShipping(
      zone: zone,
      weightBand: weightBand,
      discountBand: discountBand,
    );

    final extraCover = includeExtraCover
        ? calculateExtraCover(
            itemValueAUD: itemValueAUD,
            discountBand: discountBand,
          )
        : 0.0;

    final tariffDuties = calculateTariffDuties(
      itemValueAUD: itemValueAUD,
      countryOfOrigin: countryOfOrigin,
    );

    final zonosFees = calculateZonosFees(tariffAmount: tariffDuties);

    // Calculate subtotals
    final shippingSubtotal = ausPostShipping + extraCover;
    final dutiesSubtotal = tariffDuties + zonosFees;
    final totalShipping = shippingSubtotal + dutiesSubtotal;

    return ShippingCalculationResult(
      inputs: ShippingCalculationInput(
        itemValueAUD: itemValueAUD,
        weightBand: weightBand,
        brandName: brandName,
        countryOfOrigin: countryOfOrigin,
        tariffRate: tariffRate,
        includeExtraCover: includeExtraCover,
        discountBand: discountBand,
      ),
      breakdown: ShippingCalculationBreakdown(
        ausPostShipping: ausPostShipping,
        extraCover: extraCover,
        shippingSubtotal: shippingSubtotal,
        tariffDuties: tariffDuties,
        zonosFees: zonosFees,
        dutiesSubtotal: dutiesSubtotal,
      ),
      totalShipping: _roundTo2Decimals(totalShipping),
      warnings: ShippingCalculationWarnings(
        extraCoverRecommended: shouldWarnExtraCover(
          itemValueAUD: itemValueAUD,
          hasExtraCover: includeExtraCover,
        ),
      ),
    );
  }

  /// Helper to round to 2 decimal places
  double _roundTo2Decimals(double value) {
    return (value * 100).round() / 100;
  }
}
