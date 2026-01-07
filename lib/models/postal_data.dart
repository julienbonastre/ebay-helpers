/// Data models for postal rates, tariffs, brands, and extra cover

class WeightBand {
  final String key;
  final String label;
  final int maxWeight;
  final double basePrice;

  WeightBand({
    required this.key,
    required this.label,
    required this.maxWeight,
    required this.basePrice,
  });

  factory WeightBand.fromJson(String key, Map<String, dynamic> json) {
    return WeightBand(
      key: key,
      label: json['label'] as String,
      maxWeight: json['maxWeight'] as int,
      basePrice: (json['basePrice'] as num).toDouble(),
    );
  }
}

class PostalZone {
  final String code;
  final double handlingFee;
  final Map<int, double> discountBands;
  final Map<String, WeightBand> weightBands;

  PostalZone({
    required this.code,
    required this.handlingFee,
    required this.discountBands,
    required this.weightBands,
  });

  factory PostalZone.fromJson(String code, Map<String, dynamic> json) {
    final discountBandsJson = json['discountBands'] as Map<String, dynamic>;
    final discountBands = <int, double>{};
    discountBandsJson.forEach((key, value) {
      discountBands[int.parse(key)] = (value as num).toDouble();
    });

    final weightBandsJson = json['weightBands'] as Map<String, dynamic>;
    final weightBands = <String, WeightBand>{};
    weightBandsJson.forEach((key, value) {
      weightBands[key] = WeightBand.fromJson(key, value as Map<String, dynamic>);
    });

    return PostalZone(
      code: code,
      handlingFee: (json['handlingFee'] as num).toDouble(),
      discountBands: discountBands,
      weightBands: weightBands,
    );
  }
}

class PostalRatesData {
  final Map<String, PostalZone> zones;
  final String source;
  final String lastUpdated;

  PostalRatesData({
    required this.zones,
    required this.source,
    required this.lastUpdated,
  });

  factory PostalRatesData.fromJson(Map<String, dynamic> json) {
    final zonesJson = json['zones'] as Map<String, dynamic>;
    final zones = <String, PostalZone>{};
    zonesJson.forEach((key, value) {
      zones[key] = PostalZone.fromJson(key, value as Map<String, dynamic>);
    });

    return PostalRatesData(
      zones: zones,
      source: json['source'] as String,
      lastUpdated: json['lastUpdated'] as String,
    );
  }
}

class TariffsData {
  final Map<String, double> usaTariffRates;
  final double zonosProcessingPercent;
  final double zonosFlatFeeAUD;
  final String source;
  final String lastUpdated;

  TariffsData({
    required this.usaTariffRates,
    required this.zonosProcessingPercent,
    required this.zonosFlatFeeAUD,
    required this.source,
    required this.lastUpdated,
  });

  factory TariffsData.fromJson(Map<String, dynamic> json) {
    final ratesJson = json['usaTariffs']['rates'] as Map<String, dynamic>;
    final rates = <String, double>{};
    ratesJson.forEach((key, value) {
      rates[key] = (value as num).toDouble();
    });

    return TariffsData(
      usaTariffRates: rates,
      zonosProcessingPercent: (json['zonos']['processingChargePercent'] as num).toDouble(),
      zonosFlatFeeAUD: (json['zonos']['flatFeeAUD'] as num).toDouble(),
      source: json['source'] as String,
      lastUpdated: json['lastUpdated'] as String,
    );
  }
}

class Brand {
  final String name;
  final String primaryCOO;
  final List<String> secondaryCOO;
  final String? type;

  Brand({
    required this.name,
    required this.primaryCOO,
    required this.secondaryCOO,
    this.type,
  });

  factory Brand.fromJson(String name, Map<String, dynamic> json) {
    return Brand(
      name: name,
      primaryCOO: json['primaryCOO'] as String,
      secondaryCOO: (json['secondaryCOO'] as List<dynamic>).cast<String>(),
      type: json['type'] as String?,
    );
  }
}

class BrandsData {
  final Map<String, Brand> brands;
  final String defaultCOO;
  final String lastUpdated;

  BrandsData({
    required this.brands,
    required this.defaultCOO,
    required this.lastUpdated,
  });

  factory BrandsData.fromJson(Map<String, dynamic> json) {
    final brandsJson = json['brands'] as Map<String, dynamic>;
    final brands = <String, Brand>{};
    brandsJson.forEach((key, value) {
      brands[key] = Brand.fromJson(key, value as Map<String, dynamic>);
    });

    return BrandsData(
      brands: brands,
      defaultCOO: json['defaultCOO'] as String,
      lastUpdated: json['lastUpdated'] as String,
    );
  }
}

class ExtraCoverData {
  final double basePricePer100;
  final double thresholdAUD;
  final double warningThresholdAUD;
  final Map<int, double> discountBands;
  final String source;
  final String lastUpdated;

  ExtraCoverData({
    required this.basePricePer100,
    required this.thresholdAUD,
    required this.warningThresholdAUD,
    required this.discountBands,
    required this.source,
    required this.lastUpdated,
  });

  factory ExtraCoverData.fromJson(Map<String, dynamic> json) {
    final coverJson = json['extraCover'] as Map<String, dynamic>;
    final discountBandsJson = coverJson['discountBands'] as Map<String, dynamic>;
    final discountBands = <int, double>{};
    discountBandsJson.forEach((key, value) {
      discountBands[int.parse(key)] = (value as num).toDouble();
    });

    return ExtraCoverData(
      basePricePer100: (coverJson['basePricePer100'] as num).toDouble(),
      thresholdAUD: (coverJson['thresholdAUD'] as num).toDouble(),
      warningThresholdAUD: (coverJson['warningThresholdAUD'] as num).toDouble(),
      discountBands: discountBands,
      source: json['source'] as String,
      lastUpdated: json['lastUpdated'] as String,
    );
  }
}
