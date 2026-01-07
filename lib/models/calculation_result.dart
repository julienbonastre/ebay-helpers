/// Model for shipping calculation results

class ShippingCalculationInput {
  final double itemValueAUD;
  final String weightBand;
  final String? brandName;
  final String countryOfOrigin;
  final double tariffRate;
  final bool includeExtraCover;
  final int discountBand;

  ShippingCalculationInput({
    required this.itemValueAUD,
    required this.weightBand,
    this.brandName,
    required this.countryOfOrigin,
    required this.tariffRate,
    required this.includeExtraCover,
    required this.discountBand,
  });

  Map<String, dynamic> toJson() => {
    'itemValueAUD': itemValueAUD,
    'weightBand': weightBand,
    'brandName': brandName,
    'countryOfOrigin': countryOfOrigin,
    'tariffRate': tariffRate,
    'includeExtraCover': includeExtraCover,
    'discountBand': discountBand,
  };
}

class ShippingCalculationBreakdown {
  final double ausPostShipping;
  final double extraCover;
  final double shippingSubtotal;
  final double tariffDuties;
  final double zonosFees;
  final double dutiesSubtotal;

  ShippingCalculationBreakdown({
    required this.ausPostShipping,
    required this.extraCover,
    required this.shippingSubtotal,
    required this.tariffDuties,
    required this.zonosFees,
    required this.dutiesSubtotal,
  });

  Map<String, dynamic> toJson() => {
    'ausPostShipping': ausPostShipping,
    'extraCover': extraCover,
    'shippingSubtotal': shippingSubtotal,
    'tariffDuties': tariffDuties,
    'zonosFees': zonosFees,
    'dutiesSubtotal': dutiesSubtotal,
  };
}

class ShippingCalculationWarnings {
  final bool extraCoverRecommended;

  ShippingCalculationWarnings({
    required this.extraCoverRecommended,
  });

  Map<String, dynamic> toJson() => {
    'extraCoverRecommended': extraCoverRecommended,
  };
}

class ShippingCalculationResult {
  final ShippingCalculationInput inputs;
  final ShippingCalculationBreakdown breakdown;
  final double totalShipping;
  final ShippingCalculationWarnings warnings;

  ShippingCalculationResult({
    required this.inputs,
    required this.breakdown,
    required this.totalShipping,
    required this.warnings,
  });

  Map<String, dynamic> toJson() => {
    'inputs': inputs.toJson(),
    'breakdown': breakdown.toJson(),
    'totalShipping': totalShipping,
    'warnings': warnings.toJson(),
  };

  @override
  String toString() {
    return '''
ShippingCalculationResult:
  Item Value: \$${inputs.itemValueAUD.toStringAsFixed(2)} AUD
  Weight Band: ${inputs.weightBand}
  Country of Origin: ${inputs.countryOfOrigin} (${(inputs.tariffRate * 100).toStringAsFixed(0)}% tariff)
  Extra Cover: ${inputs.includeExtraCover ? 'Yes' : 'No'}

  Breakdown:
    AusPost Shipping: \$${breakdown.ausPostShipping.toStringAsFixed(2)}
    Extra Cover: \$${breakdown.extraCover.toStringAsFixed(2)}
    Shipping Subtotal: \$${breakdown.shippingSubtotal.toStringAsFixed(2)}
    Tariff Duties: \$${breakdown.tariffDuties.toStringAsFixed(2)}
    Zonos Fees: \$${breakdown.zonosFees.toStringAsFixed(2)}
    Duties Subtotal: \$${breakdown.dutiesSubtotal.toStringAsFixed(2)}

  TOTAL: \$${totalShipping.toStringAsFixed(2)} AUD
  ${warnings.extraCoverRecommended ? '⚠️ Extra Cover recommended for items >= \$250' : ''}
''';
  }
}
