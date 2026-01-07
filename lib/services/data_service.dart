import 'dart:convert';
import 'package:flutter/services.dart' show rootBundle;
import 'package:flutter/foundation.dart' show protected, visibleForTesting;
import '../models/postal_data.dart';

/// Service for loading and caching lookup data from assets
class DataService {
  static DataService? _instance;
  static DataService get instance => _instance ??= DataService._internal();

  DataService._internal();

  /// Constructor for testing - allows subclassing
  @visibleForTesting
  DataService.forTesting();

  PostalRatesData? _postalRates;
  TariffsData? _tariffs;
  BrandsData? _brands;
  ExtraCoverData? _extraCover;

  bool _isLoaded = false;

  bool get isLoaded => _isLoaded;

  PostalRatesData get postalRates {
    if (_postalRates == null) {
      throw StateError('Data not loaded. Call loadAll() first.');
    }
    return _postalRates!;
  }

  TariffsData get tariffs {
    if (_tariffs == null) {
      throw StateError('Data not loaded. Call loadAll() first.');
    }
    return _tariffs!;
  }

  BrandsData get brands {
    if (_brands == null) {
      throw StateError('Data not loaded. Call loadAll() first.');
    }
    return _brands!;
  }

  ExtraCoverData get extraCover {
    if (_extraCover == null) {
      throw StateError('Data not loaded. Call loadAll() first.');
    }
    return _extraCover!;
  }

  /// Load all data from assets
  Future<void> loadAll() async {
    if (_isLoaded) return;

    final results = await Future.wait([
      _loadJson('assets/data/postalRates.json'),
      _loadJson('assets/data/tariffs.json'),
      _loadJson('assets/data/brands.json'),
      _loadJson('assets/data/extraCover.json'),
    ]);

    _postalRates = PostalRatesData.fromJson(results[0]);
    _tariffs = TariffsData.fromJson(results[1]);
    _brands = BrandsData.fromJson(results[2]);
    _extraCover = ExtraCoverData.fromJson(results[3]);

    _isLoaded = true;
  }

  Future<Map<String, dynamic>> _loadJson(String path) async {
    final jsonString = await rootBundle.loadString(path);
    return json.decode(jsonString) as Map<String, dynamic>;
  }

  /// Get country of origin for a brand
  String getCountryOfOrigin(String? brandName) {
    if (brandName == null || brandName.isEmpty) {
      return brands.defaultCOO;
    }
    final brand = brands.brands[brandName];
    return brand?.primaryCOO ?? brands.defaultCOO;
  }

  /// Get tariff rate for a country
  double getTariffRate(String country) {
    return tariffs.usaTariffRates[country] ??
           tariffs.usaTariffRates[brands.defaultCOO] ??
           0.20; // Default to 20% if unknown
  }

  /// Get weight band from weight in grams
  String getWeightBandFromGrams(int weightGrams) {
    if (weightGrams < 250) return 'XSmall';
    if (weightGrams < 500) return 'Small';
    if (weightGrams < 1000) return 'Medium';
    if (weightGrams < 1500) return 'Large';
    return 'XLarge';
  }

  /// Get all available brand names
  List<String> getAvailableBrands() {
    final brandList = brands.brands.keys.toList();
    brandList.sort();
    return brandList;
  }

  /// Get all weight bands for a zone
  List<WeightBand> getWeightBands({String zone = '3-USA & Canada'}) {
    final zoneData = postalRates.zones[zone];
    if (zoneData == null) return [];
    return zoneData.weightBands.values.toList();
  }

  /// Get all countries with tariff rates
  List<MapEntry<String, double>> getTariffCountries() {
    final entries = tariffs.usaTariffRates.entries.toList();
    entries.sort((a, b) => a.key.compareTo(b.key));
    return entries;
  }

  /// Get all available zones
  List<String> getAvailableZones() {
    return postalRates.zones.keys.toList();
  }
}
