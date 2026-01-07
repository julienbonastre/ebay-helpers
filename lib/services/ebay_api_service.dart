import 'dart:convert';
import 'package:http/http.dart' as http;
import 'credentials_service.dart';

/// Service for interacting with eBay APIs
/// Uses OAuth 2.0 Client Credentials flow for API access
class EbayApiService {
  static EbayApiService? _instance;
  static EbayApiService get instance => _instance ??= EbayApiService._();

  EbayApiService._();

  final CredentialsService _credentialsService = CredentialsService.instance;

  String? _accessToken;
  DateTime? _tokenExpiry;

  /// OAuth scopes required for inventory and fulfillment operations
  static const List<String> requiredScopes = [
    'https://api.ebay.com/oauth/api_scope',
    'https://api.ebay.com/oauth/api_scope/sell.inventory',
    'https://api.ebay.com/oauth/api_scope/sell.inventory.readonly',
    'https://api.ebay.com/oauth/api_scope/sell.fulfillment',
    'https://api.ebay.com/oauth/api_scope/sell.fulfillment.readonly',
    'https://api.ebay.com/oauth/api_scope/sell.account',
    'https://api.ebay.com/oauth/api_scope/sell.account.readonly',
  ];

  /// Check if we have a valid access token
  bool get hasValidToken {
    if (_accessToken == null || _tokenExpiry == null) return false;
    return DateTime.now().isBefore(_tokenExpiry!.subtract(const Duration(minutes: 5)));
  }

  /// Get an access token using Client Credentials grant
  /// This is suitable for application-level access
  Future<String> getAccessToken() async {
    if (hasValidToken) {
      return _accessToken!;
    }

    final credentials = await _credentialsService.getCredentials();
    if (credentials == null) {
      throw EbayApiException('No credentials configured', 'AUTH_ERROR');
    }

    final authBaseUrl = await _credentialsService.getAuthBaseUrl();
    final tokenUrl = '$authBaseUrl/oauth2/token';

    final response = await http.post(
      Uri.parse(tokenUrl),
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Authorization': 'Basic ${credentials.base64Credentials}',
      },
      body: {
        'grant_type': 'client_credentials',
        'scope': requiredScopes.join(' '),
      },
    );

    if (response.statusCode != 200) {
      final error = _parseError(response);
      throw EbayApiException(
        'Failed to get access token: ${error['message']}',
        error['code'] ?? 'TOKEN_ERROR',
      );
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    _accessToken = data['access_token'] as String;
    final expiresIn = data['expires_in'] as int;
    _tokenExpiry = DateTime.now().add(Duration(seconds: expiresIn));

    return _accessToken!;
  }

  /// Clear the cached access token
  void clearToken() {
    _accessToken = null;
    _tokenExpiry = null;
  }

  /// Get fulfillment policies (shipping policies)
  Future<List<FulfillmentPolicy>> getFulfillmentPolicies({
    String marketplaceId = 'EBAY_AU',
  }) async {
    final token = await getAccessToken();
    final apiBaseUrl = await _credentialsService.getApiBaseUrl();

    final response = await http.get(
      Uri.parse('$apiBaseUrl/sell/account/v1/fulfillment_policy?marketplace_id=$marketplaceId'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
    );

    if (response.statusCode != 200) {
      final error = _parseError(response);
      throw EbayApiException(
        'Failed to get fulfillment policies: ${error['message']}',
        error['code'] ?? 'API_ERROR',
      );
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final policies = data['fulfillmentPolicies'] as List<dynamic>? ?? [];

    return policies
        .map((p) => FulfillmentPolicy.fromJson(p as Map<String, dynamic>))
        .toList();
  }

  /// Get all inventory items
  Future<InventoryItemsResponse> getInventoryItems({
    int limit = 100,
    int offset = 0,
  }) async {
    final token = await getAccessToken();
    final apiBaseUrl = await _credentialsService.getApiBaseUrl();

    final response = await http.get(
      Uri.parse('$apiBaseUrl/sell/inventory/v1/inventory_item?limit=$limit&offset=$offset'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
    );

    if (response.statusCode != 200) {
      final error = _parseError(response);
      throw EbayApiException(
        'Failed to get inventory items: ${error['message']}',
        error['code'] ?? 'API_ERROR',
      );
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return InventoryItemsResponse.fromJson(data);
  }

  /// Get all offers (active listings)
  Future<OffersResponse> getOffers({
    int limit = 100,
    int offset = 0,
    String? sku,
    String marketplaceId = 'EBAY_AU',
  }) async {
    final token = await getAccessToken();
    final apiBaseUrl = await _credentialsService.getApiBaseUrl();

    var url = '$apiBaseUrl/sell/inventory/v1/offer?limit=$limit&offset=$offset&marketplace_id=$marketplaceId';
    if (sku != null) {
      url += '&sku=${Uri.encodeComponent(sku)}';
    }

    final response = await http.get(
      Uri.parse(url),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
    );

    if (response.statusCode != 200) {
      final error = _parseError(response);
      throw EbayApiException(
        'Failed to get offers: ${error['message']}',
        error['code'] ?? 'API_ERROR',
      );
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return OffersResponse.fromJson(data);
  }

  /// Get a single offer by ID
  Future<Offer> getOffer(String offerId) async {
    final token = await getAccessToken();
    final apiBaseUrl = await _credentialsService.getApiBaseUrl();

    final response = await http.get(
      Uri.parse('$apiBaseUrl/sell/inventory/v1/offer/$offerId'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
    );

    if (response.statusCode != 200) {
      final error = _parseError(response);
      throw EbayApiException(
        'Failed to get offer: ${error['message']}',
        error['code'] ?? 'API_ERROR',
      );
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Offer.fromJson(data);
  }

  /// Update an offer (including shipping cost overrides)
  Future<void> updateOffer(String offerId, Map<String, dynamic> updateData) async {
    final token = await getAccessToken();
    final apiBaseUrl = await _credentialsService.getApiBaseUrl();

    final response = await http.put(
      Uri.parse('$apiBaseUrl/sell/inventory/v1/offer/$offerId'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
      body: jsonEncode(updateData),
    );

    if (response.statusCode != 200 && response.statusCode != 204) {
      final error = _parseError(response);
      throw EbayApiException(
        'Failed to update offer: ${error['message']}',
        error['code'] ?? 'API_ERROR',
      );
    }
  }

  /// Test API connection
  Future<bool> testConnection() async {
    try {
      await getAccessToken();
      return true;
    } catch (e) {
      return false;
    }
  }

  Map<String, String> _parseError(http.Response response) {
    try {
      final data = jsonDecode(response.body);
      if (data is Map<String, dynamic>) {
        final errors = data['errors'] as List<dynamic>?;
        if (errors != null && errors.isNotEmpty) {
          final firstError = errors.first as Map<String, dynamic>;
          return {
            'message': firstError['message'] as String? ?? 'Unknown error',
            'code': firstError['errorId']?.toString() ?? response.statusCode.toString(),
          };
        }
        return {
          'message': data['error_description'] as String? ??
                     data['error'] as String? ??
                     'Unknown error',
          'code': response.statusCode.toString(),
        };
      }
    } catch (_) {}
    return {
      'message': 'HTTP ${response.statusCode}: ${response.reasonPhrase}',
      'code': response.statusCode.toString(),
    };
  }
}

/// Custom exception for eBay API errors
class EbayApiException implements Exception {
  final String message;
  final String code;

  EbayApiException(this.message, this.code);

  @override
  String toString() => 'EbayApiException [$code]: $message';
}

/// Fulfillment (shipping) policy model
class FulfillmentPolicy {
  final String fulfillmentPolicyId;
  final String name;
  final String? description;
  final String marketplaceId;
  final List<ShippingOption> shippingOptions;

  FulfillmentPolicy({
    required this.fulfillmentPolicyId,
    required this.name,
    this.description,
    required this.marketplaceId,
    required this.shippingOptions,
  });

  factory FulfillmentPolicy.fromJson(Map<String, dynamic> json) {
    final options = json['shippingOptions'] as List<dynamic>? ?? [];
    return FulfillmentPolicy(
      fulfillmentPolicyId: json['fulfillmentPolicyId'] as String,
      name: json['name'] as String,
      description: json['description'] as String?,
      marketplaceId: json['marketplaceId'] as String,
      shippingOptions: options
          .map((o) => ShippingOption.fromJson(o as Map<String, dynamic>))
          .toList(),
    );
  }
}

/// Shipping option within a fulfillment policy
class ShippingOption {
  final String optionType; // DOMESTIC or INTERNATIONAL
  final List<ShippingService> shippingServices;

  ShippingOption({
    required this.optionType,
    required this.shippingServices,
  });

  factory ShippingOption.fromJson(Map<String, dynamic> json) {
    final services = json['shippingServices'] as List<dynamic>? ?? [];
    return ShippingOption(
      optionType: json['optionType'] as String,
      shippingServices: services
          .map((s) => ShippingService.fromJson(s as Map<String, dynamic>))
          .toList(),
    );
  }
}

/// Shipping service details
class ShippingService {
  final int sortOrder;
  final String shippingCarrierCode;
  final String shippingServiceCode;
  final Amount? shippingCost;
  final Amount? additionalShippingCost;
  final List<String>? shipToLocations;

  ShippingService({
    required this.sortOrder,
    required this.shippingCarrierCode,
    required this.shippingServiceCode,
    this.shippingCost,
    this.additionalShippingCost,
    this.shipToLocations,
  });

  factory ShippingService.fromJson(Map<String, dynamic> json) {
    return ShippingService(
      sortOrder: json['sortOrder'] as int? ?? 0,
      shippingCarrierCode: json['shippingCarrierCode'] as String? ?? '',
      shippingServiceCode: json['shippingServiceCode'] as String? ?? '',
      shippingCost: json['shippingCost'] != null
          ? Amount.fromJson(json['shippingCost'] as Map<String, dynamic>)
          : null,
      additionalShippingCost: json['additionalShippingCost'] != null
          ? Amount.fromJson(json['additionalShippingCost'] as Map<String, dynamic>)
          : null,
      shipToLocations: (json['shipToLocations']?['regionIncluded'] as List<dynamic>?)
          ?.map((r) => (r as Map<String, dynamic>)['regionName'] as String)
          .toList(),
    );
  }
}

/// Money amount
class Amount {
  final String currency;
  final String value;

  Amount({required this.currency, required this.value});

  factory Amount.fromJson(Map<String, dynamic> json) {
    return Amount(
      currency: json['currency'] as String,
      value: json['value'] as String,
    );
  }

  Map<String, dynamic> toJson() => {
    'currency': currency,
    'value': value,
  };

  double get numericValue => double.tryParse(value) ?? 0.0;
}

/// Inventory items response
class InventoryItemsResponse {
  final List<InventoryItem> inventoryItems;
  final int total;
  final int limit;
  final int offset;

  InventoryItemsResponse({
    required this.inventoryItems,
    required this.total,
    required this.limit,
    required this.offset,
  });

  factory InventoryItemsResponse.fromJson(Map<String, dynamic> json) {
    final items = json['inventoryItems'] as List<dynamic>? ?? [];
    return InventoryItemsResponse(
      inventoryItems: items
          .map((i) => InventoryItem.fromJson(i as Map<String, dynamic>))
          .toList(),
      total: json['total'] as int? ?? 0,
      limit: json['limit'] as int? ?? 100,
      offset: json['offset'] as int? ?? 0,
    );
  }
}

/// Inventory item model
class InventoryItem {
  final String sku;
  final String? condition;
  final Product? product;

  InventoryItem({
    required this.sku,
    this.condition,
    this.product,
  });

  factory InventoryItem.fromJson(Map<String, dynamic> json) {
    return InventoryItem(
      sku: json['sku'] as String,
      condition: json['condition'] as String?,
      product: json['product'] != null
          ? Product.fromJson(json['product'] as Map<String, dynamic>)
          : null,
    );
  }
}

/// Product details
class Product {
  final String? title;
  final String? description;
  final String? brand;
  final List<String>? imageUrls;

  Product({
    this.title,
    this.description,
    this.brand,
    this.imageUrls,
  });

  factory Product.fromJson(Map<String, dynamic> json) {
    return Product(
      title: json['title'] as String?,
      description: json['description'] as String?,
      brand: json['brand'] as String?,
      imageUrls: (json['imageUrls'] as List<dynamic>?)?.cast<String>(),
    );
  }
}

/// Offers response
class OffersResponse {
  final List<Offer> offers;
  final int total;
  final int limit;
  final int offset;

  OffersResponse({
    required this.offers,
    required this.total,
    required this.limit,
    required this.offset,
  });

  factory OffersResponse.fromJson(Map<String, dynamic> json) {
    final offers = json['offers'] as List<dynamic>? ?? [];
    return OffersResponse(
      offers: offers
          .map((o) => Offer.fromJson(o as Map<String, dynamic>))
          .toList(),
      total: json['total'] as int? ?? 0,
      limit: json['limit'] as int? ?? 100,
      offset: json['offset'] as int? ?? 0,
    );
  }
}

/// Offer (active listing) model
class Offer {
  final String offerId;
  final String sku;
  final String? listingId;
  final String status;
  final String marketplaceId;
  final String format;
  final Amount? pricingSummary;
  final ListingPolicies? listingPolicies;

  Offer({
    required this.offerId,
    required this.sku,
    this.listingId,
    required this.status,
    required this.marketplaceId,
    required this.format,
    this.pricingSummary,
    this.listingPolicies,
  });

  factory Offer.fromJson(Map<String, dynamic> json) {
    return Offer(
      offerId: json['offerId'] as String,
      sku: json['sku'] as String,
      listingId: json['listing']?['listingId'] as String?,
      status: json['status'] as String,
      marketplaceId: json['marketplaceId'] as String,
      format: json['format'] as String,
      pricingSummary: json['pricingSummary']?['price'] != null
          ? Amount.fromJson(json['pricingSummary']['price'] as Map<String, dynamic>)
          : null,
      listingPolicies: json['listingPolicies'] != null
          ? ListingPolicies.fromJson(json['listingPolicies'] as Map<String, dynamic>)
          : null,
    );
  }
}

/// Listing policies including shipping cost overrides
class ListingPolicies {
  final String? fulfillmentPolicyId;
  final String? paymentPolicyId;
  final String? returnPolicyId;
  final List<ShippingCostOverride>? shippingCostOverrides;

  ListingPolicies({
    this.fulfillmentPolicyId,
    this.paymentPolicyId,
    this.returnPolicyId,
    this.shippingCostOverrides,
  });

  factory ListingPolicies.fromJson(Map<String, dynamic> json) {
    final overrides = json['shippingCostOverrides'] as List<dynamic>?;
    return ListingPolicies(
      fulfillmentPolicyId: json['fulfillmentPolicyId'] as String?,
      paymentPolicyId: json['paymentPolicyId'] as String?,
      returnPolicyId: json['returnPolicyId'] as String?,
      shippingCostOverrides: overrides
          ?.map((o) => ShippingCostOverride.fromJson(o as Map<String, dynamic>))
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
    if (fulfillmentPolicyId != null) 'fulfillmentPolicyId': fulfillmentPolicyId,
    if (paymentPolicyId != null) 'paymentPolicyId': paymentPolicyId,
    if (returnPolicyId != null) 'returnPolicyId': returnPolicyId,
    if (shippingCostOverrides != null)
      'shippingCostOverrides': shippingCostOverrides!.map((o) => o.toJson()).toList(),
  };
}

/// Shipping cost override for a specific shipping service
class ShippingCostOverride {
  final String shippingServiceType; // DOMESTIC or INTERNATIONAL
  final int priority; // Maps to sortOrderId in fulfillment policy
  final Amount? shippingCost;
  final Amount? additionalShippingCost;

  ShippingCostOverride({
    required this.shippingServiceType,
    required this.priority,
    this.shippingCost,
    this.additionalShippingCost,
  });

  factory ShippingCostOverride.fromJson(Map<String, dynamic> json) {
    return ShippingCostOverride(
      shippingServiceType: json['shippingServiceType'] as String,
      priority: json['priority'] as int,
      shippingCost: json['shippingCost'] != null
          ? Amount.fromJson(json['shippingCost'] as Map<String, dynamic>)
          : null,
      additionalShippingCost: json['additionalShippingCost'] != null
          ? Amount.fromJson(json['additionalShippingCost'] as Map<String, dynamic>)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'shippingServiceType': shippingServiceType,
    'priority': priority,
    if (shippingCost != null) 'shippingCost': shippingCost!.toJson(),
    if (additionalShippingCost != null)
      'additionalShippingCost': additionalShippingCost!.toJson(),
  };

  /// Create an override for US shipping
  static ShippingCostOverride forUSShipping({
    required double shippingCostAUD,
    int priority = 1,
  }) {
    return ShippingCostOverride(
      shippingServiceType: 'INTERNATIONAL',
      priority: priority,
      shippingCost: Amount(
        currency: 'AUD',
        value: shippingCostAUD.toStringAsFixed(2),
      ),
    );
  }
}
