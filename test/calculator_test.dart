import 'package:flutter_test/flutter_test.dart';
import 'package:ebay_postage_helper/models/postal_data.dart';
import 'package:ebay_postage_helper/models/calculation_result.dart';
import 'package:ebay_postage_helper/services/calculator_service.dart';
import 'package:ebay_postage_helper/services/data_service.dart';

/// Unit tests for the calculator service
/// Test cases derived from TariffAndPostalCalculator.xlsx spreadsheet
///
/// Spreadsheet reference values (Zone 3 - USA & Canada):
/// - XSmall (<250g): $22.30 base
/// - Small (250-500g): $29.00 base
/// - Medium (500-1kg): $42.20 base
/// - Large (1-1.5kg): $55.55 base
/// - XLarge (1.5-2kg): $68.85 base
/// - Handling fee: 2%
///
/// Tariff rates:
/// - China: 20%, India: 50%, Indonesia: 19%, Vietnam: 20%
/// - Mexico: 25%, Australia: 10%, Japan: 15%, USA: 0%
///
/// Zonos: 10% of duties + $1.69 flat
/// Extra Cover: $4 per $100 above $100 threshold

void main() {
  late TestDataService dataService;
  late CalculatorService calculator;

  setUp(() {
    dataService = TestDataService();
    calculator = CalculatorService(dataService: dataService);
  });

  group('AusPost Shipping Calculations', () {
    test('XSmall base rate with handling fee', () {
      // Base: $22.30, Handling: 2%, No discount
      // Expected: 22.30 * 1.02 = 22.746 ≈ 22.75
      final result = calculator.calculateAusPostShipping(
        zone: '3-USA & Canada',
        weightBand: 'XSmall',
        discountBand: 0,
      );
      expect(result, closeTo(22.75, 0.01));
    });

    test('Small base rate with handling fee', () {
      // Base: $29.00, Handling: 2%, No discount
      // Expected: 29.00 * 1.02 = 29.58
      final result = calculator.calculateAusPostShipping(
        zone: '3-USA & Canada',
        weightBand: 'Small',
        discountBand: 0,
      );
      expect(result, closeTo(29.58, 0.01));
    });

    test('Medium base rate with handling fee', () {
      // Base: $42.20, Handling: 2%, No discount
      // Expected: 42.20 * 1.02 = 43.044 ≈ 43.04
      final result = calculator.calculateAusPostShipping(
        zone: '3-USA & Canada',
        weightBand: 'Medium',
        discountBand: 0,
      );
      expect(result, closeTo(43.04, 0.01));
    });

    test('Large base rate with handling fee', () {
      // Base: $55.55, Handling: 2%, No discount
      // Expected: 55.55 * 1.02 = 56.661 ≈ 56.66
      final result = calculator.calculateAusPostShipping(
        zone: '3-USA & Canada',
        weightBand: 'Large',
        discountBand: 0,
      );
      expect(result, closeTo(56.66, 0.01));
    });

    test('XLarge base rate with handling fee', () {
      // Base: $68.85, Handling: 2%, No discount
      // Expected: 68.85 * 1.02 = 70.227 ≈ 70.23
      final result = calculator.calculateAusPostShipping(
        zone: '3-USA & Canada',
        weightBand: 'XLarge',
        discountBand: 0,
      );
      expect(result, closeTo(70.23, 0.01));
    });

    test('Medium with 5% discount (Band 1)', () {
      // Base: $42.20, Handling: 2%, Discount: 5%
      // Expected: 42.20 * 1.02 * 0.95 = 40.8918 ≈ 40.89
      final result = calculator.calculateAusPostShipping(
        zone: '3-USA & Canada',
        weightBand: 'Medium',
        discountBand: 1,
      );
      expect(result, closeTo(40.89, 0.01));
    });

    test('Medium with 15% discount (Band 2)', () {
      // Base: $42.20, Handling: 2%, Discount: 15%
      // Expected: 42.20 * 1.02 * 0.85 = 36.5874 ≈ 36.59
      final result = calculator.calculateAusPostShipping(
        zone: '3-USA & Canada',
        weightBand: 'Medium',
        discountBand: 2,
      );
      expect(result, closeTo(36.59, 0.01));
    });
  });

  group('Tariff Duty Calculations', () {
    test('China tariff (20%)', () {
      // $100 item * 20% = $20
      final result = calculator.calculateTariffDuties(
        itemValueAUD: 100,
        countryOfOrigin: 'China',
      );
      expect(result, closeTo(20.00, 0.01));
    });

    test('India tariff (50%)', () {
      // $100 item * 50% = $50
      final result = calculator.calculateTariffDuties(
        itemValueAUD: 100,
        countryOfOrigin: 'India',
      );
      expect(result, closeTo(50.00, 0.01));
    });

    test('Indonesia tariff (19%)', () {
      // $200 item * 19% = $38
      final result = calculator.calculateTariffDuties(
        itemValueAUD: 200,
        countryOfOrigin: 'Indonesia',
      );
      expect(result, closeTo(38.00, 0.01));
    });

    test('Australia tariff (10%)', () {
      // $500 item * 10% = $50
      final result = calculator.calculateTariffDuties(
        itemValueAUD: 500,
        countryOfOrigin: 'Australia',
      );
      expect(result, closeTo(50.00, 0.01));
    });

    test('Mexico tariff (25%)', () {
      // $300 item * 25% = $75
      final result = calculator.calculateTariffDuties(
        itemValueAUD: 300,
        countryOfOrigin: 'Mexico',
      );
      expect(result, closeTo(75.00, 0.01));
    });

    test('USA origin (0% tariff)', () {
      // $100 item * 0% = $0
      final result = calculator.calculateTariffDuties(
        itemValueAUD: 100,
        countryOfOrigin: 'United States',
      );
      expect(result, closeTo(0.00, 0.01));
    });
  });

  group('Zonos Fee Calculations', () {
    test('Zonos fees on \$20 duties', () {
      // 10% of $20 + $1.69 = $2 + $1.69 = $3.69
      final result = calculator.calculateZonosFees(tariffAmount: 20.00);
      expect(result, closeTo(3.69, 0.01));
    });

    test('Zonos fees on \$50 duties', () {
      // 10% of $50 + $1.69 = $5 + $1.69 = $6.69
      final result = calculator.calculateZonosFees(tariffAmount: 50.00);
      expect(result, closeTo(6.69, 0.01));
    });

    test('Zonos fees on zero duties', () {
      // 10% of $0 + $1.69 = $1.69
      final result = calculator.calculateZonosFees(tariffAmount: 0.00);
      expect(result, closeTo(1.69, 0.01));
    });
  });

  group('Extra Cover Calculations', () {
    test('No extra cover for items <= \$100', () {
      final result = calculator.calculateExtraCover(itemValueAUD: 100);
      expect(result, equals(0.0));
    });

    test('No extra cover for items < \$100', () {
      final result = calculator.calculateExtraCover(itemValueAUD: 50);
      expect(result, equals(0.0));
    });

    test('Extra cover for \$200 item (Band 0)', () {
      // ($200 - $100) / 100 * $4 = 1 * $4 = $4
      final result = calculator.calculateExtraCover(
        itemValueAUD: 200,
        discountBand: 0,
      );
      expect(result, closeTo(4.00, 0.01));
    });

    test('Extra cover for \$500 item (Band 0)', () {
      // ($500 - $100) / 100 * $4 = 4 * $4 = $16
      final result = calculator.calculateExtraCover(
        itemValueAUD: 500,
        discountBand: 0,
      );
      expect(result, closeTo(16.00, 0.01));
    });

    test('Extra cover for \$500 item with 40% discount (Band 1)', () {
      // ($500 - $100) / 100 * $4 * 0.6 = 4 * $4 * 0.6 = $9.60
      final result = calculator.calculateExtraCover(
        itemValueAUD: 500,
        discountBand: 1,
      );
      expect(result, closeTo(9.60, 0.01));
    });
  });

  group('Extra Cover Warning', () {
    test('Warning shown for \$250+ item without cover', () {
      final result = calculator.shouldWarnExtraCover(
        itemValueAUD: 250,
        hasExtraCover: false,
      );
      expect(result, isTrue);
    });

    test('No warning for \$250+ item with cover', () {
      final result = calculator.shouldWarnExtraCover(
        itemValueAUD: 300,
        hasExtraCover: true,
      );
      expect(result, isFalse);
    });

    test('No warning for item under \$250', () {
      final result = calculator.shouldWarnExtraCover(
        itemValueAUD: 200,
        hasExtraCover: false,
      );
      expect(result, isFalse);
    });
  });

  group('Full USA Shipping Calculation', () {
    test('Spreadsheet example: \$125 Medium, Kip & Co (India), No Extra Cover', () {
      // From spreadsheet "Lookup Calculator" with:
      // Item Value: $125, Weight: Medium, Brand: Kip & Co (India), No Extra Cover
      //
      // Expected breakdown:
      // AusPost: 42.20 * 1.02 = 43.04
      // Extra Cover: 0 (not selected)
      // Tariff (India 50%): 125 * 0.50 = 62.50
      // Zonos: 62.50 * 0.10 + 1.69 = 6.25 + 1.69 = 7.94
      // Total: 43.04 + 0 + 62.50 + 7.94 = 113.48

      final result = calculator.calculateUSAShipping(
        itemValueAUD: 125,
        weightBand: 'Medium',
        brandName: 'Kip & Co',
        includeExtraCover: false,
        discountBand: 0,
      );

      expect(result.inputs.countryOfOrigin, equals('India'));
      expect(result.inputs.tariffRate, equals(0.50));
      expect(result.breakdown.ausPostShipping, closeTo(43.04, 0.01));
      expect(result.breakdown.extraCover, equals(0.0));
      expect(result.breakdown.tariffDuties, closeTo(62.50, 0.01));
      expect(result.breakdown.zonosFees, closeTo(7.94, 0.01));
      expect(result.totalShipping, closeTo(113.48, 0.01));
    });

    test('\$300 Large, China origin, with Extra Cover', () {
      // AusPost: 55.55 * 1.02 = 56.66
      // Extra Cover: ($300 - $100) / 100 * $4 = 8.00
      // Tariff (China 20%): 300 * 0.20 = 60.00
      // Zonos: 60 * 0.10 + 1.69 = 7.69
      // Total: 56.66 + 8.00 + 60.00 + 7.69 = 132.35

      final result = calculator.calculateUSAShipping(
        itemValueAUD: 300,
        weightBand: 'Large',
        countryOfOriginOverride: 'China',
        includeExtraCover: true,
        discountBand: 0,
      );

      expect(result.breakdown.ausPostShipping, closeTo(56.66, 0.01));
      expect(result.breakdown.extraCover, closeTo(8.00, 0.01));
      expect(result.breakdown.tariffDuties, closeTo(60.00, 0.01));
      expect(result.breakdown.zonosFees, closeTo(7.69, 0.01));
      expect(result.totalShipping, closeTo(132.35, 0.01));
      expect(result.warnings.extraCoverRecommended, isFalse);
    });

    test('\$500 XLarge, Indonesia (Ada + Lou), no Extra Cover - shows warning', () {
      // AusPost: 68.85 * 1.02 = 70.23
      // Extra Cover: 0 (not selected)
      // Tariff (Indonesia 19%): 500 * 0.19 = 95.00
      // Zonos: 95 * 0.10 + 1.69 = 11.19
      // Total: 70.23 + 0 + 95.00 + 11.19 = 176.42

      final result = calculator.calculateUSAShipping(
        itemValueAUD: 500,
        weightBand: 'XLarge',
        brandName: 'Ada + Lou',
        includeExtraCover: false,
        discountBand: 0,
      );

      expect(result.inputs.countryOfOrigin, equals('Indonesia'));
      expect(result.breakdown.ausPostShipping, closeTo(70.23, 0.01));
      expect(result.breakdown.tariffDuties, closeTo(95.00, 0.01));
      expect(result.breakdown.zonosFees, closeTo(11.19, 0.01));
      expect(result.totalShipping, closeTo(176.42, 0.01));
      expect(result.warnings.extraCoverRecommended, isTrue); // $500 >= $250
    });

    test('\$100 Small, Australian made (Ghanda), no Extra Cover', () {
      // AusPost: 29.00 * 1.02 = 29.58
      // Extra Cover: 0
      // Tariff (Australia 10%): 100 * 0.10 = 10.00
      // Zonos: 10 * 0.10 + 1.69 = 2.69
      // Total: 29.58 + 0 + 10.00 + 2.69 = 42.27

      final result = calculator.calculateUSAShipping(
        itemValueAUD: 100,
        weightBand: 'Small',
        brandName: 'Ghanda',
        includeExtraCover: false,
        discountBand: 0,
      );

      expect(result.inputs.countryOfOrigin, equals('Australia'));
      expect(result.inputs.tariffRate, equals(0.10));
      expect(result.breakdown.ausPostShipping, closeTo(29.58, 0.01));
      expect(result.breakdown.tariffDuties, closeTo(10.00, 0.01));
      expect(result.breakdown.zonosFees, closeTo(2.69, 0.01));
      expect(result.totalShipping, closeTo(42.27, 0.01));
    });

    test('US-made item (Lele Sadoughi) has 0% tariff', () {
      final result = calculator.calculateUSAShipping(
        itemValueAUD: 200,
        weightBand: 'XSmall',
        brandName: 'Lele Sadoughi',
        includeExtraCover: false,
        discountBand: 0,
      );

      expect(result.inputs.countryOfOrigin, equals('United States'));
      expect(result.inputs.tariffRate, equals(0.0));
      expect(result.breakdown.tariffDuties, equals(0.0));
      // Only Zonos flat fee applies: $1.69
      expect(result.breakdown.zonosFees, closeTo(1.69, 0.01));
    });
  });

  group('Brand to Country Mapping', () {
    test('Kip & Co maps to India', () {
      expect(dataService.getCountryOfOrigin('Kip & Co'), equals('India'));
    });

    test('Spell maps to China', () {
      expect(dataService.getCountryOfOrigin('Spell'), equals('China'));
    });

    test('Ada + Lou maps to Indonesia', () {
      expect(dataService.getCountryOfOrigin('Ada + Lou'), equals('Indonesia'));
    });

    test('Ghanda maps to Australia', () {
      expect(dataService.getCountryOfOrigin('Ghanda'), equals('Australia'));
    });

    test("Jen's Pirate Booty maps to Mexico", () {
      expect(dataService.getCountryOfOrigin("Jen's Pirate Booty"), equals('Mexico'));
    });

    test('Lele Sadoughi maps to United States', () {
      expect(dataService.getCountryOfOrigin('Lele Sadoughi'), equals('United States'));
    });

    test('Unknown brand defaults to China', () {
      expect(dataService.getCountryOfOrigin('Unknown Brand'), equals('China'));
    });

    test('Null brand defaults to China', () {
      expect(dataService.getCountryOfOrigin(null), equals('China'));
    });
  });

  group('Weight Band from Grams', () {
    test('100g maps to XSmall', () {
      expect(dataService.getWeightBandFromGrams(100), equals('XSmall'));
    });

    test('249g maps to XSmall', () {
      expect(dataService.getWeightBandFromGrams(249), equals('XSmall'));
    });

    test('250g maps to Small', () {
      expect(dataService.getWeightBandFromGrams(250), equals('Small'));
    });

    test('499g maps to Small', () {
      expect(dataService.getWeightBandFromGrams(499), equals('Small'));
    });

    test('500g maps to Medium', () {
      expect(dataService.getWeightBandFromGrams(500), equals('Medium'));
    });

    test('999g maps to Medium', () {
      expect(dataService.getWeightBandFromGrams(999), equals('Medium'));
    });

    test('1000g maps to Large', () {
      expect(dataService.getWeightBandFromGrams(1000), equals('Large'));
    });

    test('1499g maps to Large', () {
      expect(dataService.getWeightBandFromGrams(1499), equals('Large'));
    });

    test('1500g maps to XLarge', () {
      expect(dataService.getWeightBandFromGrams(1500), equals('XLarge'));
    });

    test('2000g maps to XLarge', () {
      expect(dataService.getWeightBandFromGrams(2000), equals('XLarge'));
    });
  });
}

/// Test implementation of DataService with hardcoded data
/// This avoids needing to load assets during tests
class TestDataService extends DataService {
  TestDataService() : super.forTesting();

  @override
  bool get isLoaded => true;

  @override
  PostalRatesData get postalRates => PostalRatesData(
    zones: {
      '3-USA & Canada': PostalZone(
        code: '3-USA & Canada',
        handlingFee: 0.02,
        discountBands: {0: 0, 1: 0.05, 2: 0.15, 3: 0.20, 4: 0.25, 5: 0.30},
        weightBands: {
          'XSmall': WeightBand(key: 'XSmall', label: 'XSmall [< 250g]', maxWeight: 250, basePrice: 22.30),
          'Small': WeightBand(key: 'Small', label: 'Small [250 - 500g]', maxWeight: 500, basePrice: 29.00),
          'Medium': WeightBand(key: 'Medium', label: 'Medium [500 - 1kg]', maxWeight: 1000, basePrice: 42.20),
          'Large': WeightBand(key: 'Large', label: 'Large [1 - 1.5kg]', maxWeight: 1500, basePrice: 55.55),
          'XLarge': WeightBand(key: 'XLarge', label: 'XLarge [1.5kg - 2kg]', maxWeight: 2000, basePrice: 68.85),
        },
      ),
    },
    source: 'Test Data',
    lastUpdated: '2025-01-07',
  );

  @override
  TariffsData get tariffs => TariffsData(
    usaTariffRates: {
      'China': 0.20,
      'Malaysia': 0.19,
      'Indonesia': 0.19,
      'Vietnam': 0.20,
      'Japan': 0.15,
      'India': 0.50,
      'Mexico': 0.25,
      'Australia': 0.10,
      'United States': 0.0,
    },
    zonosProcessingPercent: 0.10,
    zonosFlatFeeAUD: 1.69,
    source: 'Test Data',
    lastUpdated: '2025-01-07',
  );

  @override
  BrandsData get brands => BrandsData(
    brands: {
      'Ada + Lou': Brand(name: 'Ada + Lou', primaryCOO: 'Indonesia', secondaryCOO: []),
      'Ghanda': Brand(name: 'Ghanda', primaryCOO: 'Australia', secondaryCOO: []),
      'Kip & Co': Brand(name: 'Kip & Co', primaryCOO: 'India', secondaryCOO: []),
      'Spell': Brand(name: 'Spell', primaryCOO: 'China', secondaryCOO: []),
      "Jen's Pirate Booty": Brand(name: "Jen's Pirate Booty", primaryCOO: 'Mexico', secondaryCOO: []),
      'Lele Sadoughi': Brand(name: 'Lele Sadoughi', primaryCOO: 'United States', secondaryCOO: [], type: 'Headbands'),
    },
    defaultCOO: 'China',
    lastUpdated: '2025-01-07',
  );

  @override
  ExtraCoverData get extraCover => ExtraCoverData(
    basePricePer100: 4.00,
    thresholdAUD: 100,
    warningThresholdAUD: 250,
    discountBands: {0: 0, 1: 0.40, 2: 0.40, 3: 0.40, 4: 0.40, 5: 0.40},
    source: 'Test Data',
    lastUpdated: '2025-01-07',
  );
}
