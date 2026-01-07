import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';

/// Service for securely storing and retrieving eBay API credentials
/// Stores credentials in local storage (SharedPreferences)
/// Note: For production, consider flutter_secure_storage for encryption
class CredentialsService {
  static const String _keyPrefix = 'ebay_postage_helper_';
  static const String _credentialsKey = '${_keyPrefix}credentials';
  static const String _environmentKey = '${_keyPrefix}environment';

  static CredentialsService? _instance;
  static CredentialsService get instance => _instance ??= CredentialsService._();

  CredentialsService._();

  EbayCredentials? _cachedCredentials;
  EbayEnvironment? _cachedEnvironment;

  /// Check if credentials have been configured
  Future<bool> hasCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.containsKey(_credentialsKey);
  }

  /// Get stored credentials
  Future<EbayCredentials?> getCredentials() async {
    if (_cachedCredentials != null) {
      return _cachedCredentials;
    }

    final prefs = await SharedPreferences.getInstance();
    final jsonStr = prefs.getString(_credentialsKey);
    if (jsonStr == null) return null;

    try {
      final json = jsonDecode(jsonStr) as Map<String, dynamic>;
      _cachedCredentials = EbayCredentials.fromJson(json);
      return _cachedCredentials;
    } catch (e) {
      // If stored data is corrupted, clear it
      await clearCredentials();
      return null;
    }
  }

  /// Save credentials
  Future<void> saveCredentials(EbayCredentials credentials) async {
    final prefs = await SharedPreferences.getInstance();
    final jsonStr = jsonEncode(credentials.toJson());
    await prefs.setString(_credentialsKey, jsonStr);
    _cachedCredentials = credentials;
  }

  /// Clear stored credentials
  Future<void> clearCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_credentialsKey);
    _cachedCredentials = null;
  }

  /// Get stored environment setting
  Future<EbayEnvironment> getEnvironment() async {
    if (_cachedEnvironment != null) {
      return _cachedEnvironment!;
    }

    final prefs = await SharedPreferences.getInstance();
    final envStr = prefs.getString(_environmentKey);
    _cachedEnvironment = envStr == 'production'
        ? EbayEnvironment.production
        : EbayEnvironment.sandbox;
    return _cachedEnvironment!;
  }

  /// Save environment setting
  Future<void> saveEnvironment(EbayEnvironment environment) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      _environmentKey,
      environment == EbayEnvironment.production ? 'production' : 'sandbox',
    );
    _cachedEnvironment = environment;
  }

  /// Get the appropriate API base URL for the current environment
  Future<String> getApiBaseUrl() async {
    final env = await getEnvironment();
    return env == EbayEnvironment.production
        ? 'https://api.ebay.com'
        : 'https://api.sandbox.ebay.com';
  }

  /// Get the appropriate auth URL for the current environment
  Future<String> getAuthBaseUrl() async {
    final env = await getEnvironment();
    return env == EbayEnvironment.production
        ? 'https://auth.ebay.com'
        : 'https://auth.sandbox.ebay.com';
  }
}

/// eBay API credentials
class EbayCredentials {
  final String appId;      // Client ID
  final String certId;     // Client Secret
  final String? devId;     // Developer ID (optional for most APIs)
  final String? ruName;    // Redirect URL name for OAuth

  EbayCredentials({
    required this.appId,
    required this.certId,
    this.devId,
    this.ruName,
  });

  factory EbayCredentials.fromJson(Map<String, dynamic> json) {
    return EbayCredentials(
      appId: json['appId'] as String,
      certId: json['certId'] as String,
      devId: json['devId'] as String?,
      ruName: json['ruName'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'appId': appId,
    'certId': certId,
    if (devId != null) 'devId': devId,
    if (ruName != null) 'ruName': ruName,
  };

  /// Get base64 encoded credentials for Authorization header
  String get base64Credentials {
    final credentials = '$appId:$certId';
    return base64Encode(utf8.encode(credentials));
  }

  /// Mask the cert ID for display purposes
  String get maskedCertId {
    if (certId.length <= 8) return '****';
    return '${certId.substring(0, 4)}...${certId.substring(certId.length - 4)}';
  }
}

/// eBay API environment
enum EbayEnvironment {
  sandbox,
  production,
}

extension EbayEnvironmentExtension on EbayEnvironment {
  String get displayName {
    switch (this) {
      case EbayEnvironment.sandbox:
        return 'Sandbox (Testing)';
      case EbayEnvironment.production:
        return 'Production (Live)';
    }
  }

  String get shortName {
    switch (this) {
      case EbayEnvironment.sandbox:
        return 'Sandbox';
      case EbayEnvironment.production:
        return 'Production';
    }
  }
}
