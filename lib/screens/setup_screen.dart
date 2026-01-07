import 'package:flutter/material.dart';
import '../services/credentials_service.dart';
import '../services/ebay_api_service.dart';

/// Setup screen for entering eBay API credentials
/// Shown on first launch or when credentials are not configured
class SetupScreen extends StatefulWidget {
  final VoidCallback onSetupComplete;

  const SetupScreen({super.key, required this.onSetupComplete});

  @override
  State<SetupScreen> createState() => _SetupScreenState();
}

class _SetupScreenState extends State<SetupScreen> {
  final _formKey = GlobalKey<FormState>();
  final _appIdController = TextEditingController();
  final _certIdController = TextEditingController();
  final _devIdController = TextEditingController();
  final _ruNameController = TextEditingController();

  EbayEnvironment _selectedEnvironment = EbayEnvironment.sandbox;
  bool _isLoading = false;
  bool _isTesting = false;
  String? _errorMessage;
  String? _successMessage;
  bool _obscureCertId = true;

  @override
  void initState() {
    super.initState();
    _loadExistingCredentials();
  }

  Future<void> _loadExistingCredentials() async {
    final credentials = await CredentialsService.instance.getCredentials();
    final environment = await CredentialsService.instance.getEnvironment();

    if (credentials != null) {
      setState(() {
        _appIdController.text = credentials.appId;
        _certIdController.text = credentials.certId;
        _devIdController.text = credentials.devId ?? '';
        _ruNameController.text = credentials.ruName ?? '';
        _selectedEnvironment = environment;
      });
    }
  }

  Future<void> _testConnection() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _isTesting = true;
      _errorMessage = null;
      _successMessage = null;
    });

    try {
      // Save credentials temporarily for testing
      final credentials = EbayCredentials(
        appId: _appIdController.text.trim(),
        certId: _certIdController.text.trim(),
        devId: _devIdController.text.trim().isEmpty ? null : _devIdController.text.trim(),
        ruName: _ruNameController.text.trim().isEmpty ? null : _ruNameController.text.trim(),
      );

      await CredentialsService.instance.saveCredentials(credentials);
      await CredentialsService.instance.saveEnvironment(_selectedEnvironment);

      // Clear any cached token
      EbayApiService.instance.clearToken();

      // Test the connection
      final success = await EbayApiService.instance.testConnection();

      setState(() {
        if (success) {
          _successMessage = 'Connection successful! API credentials are valid.';
        } else {
          _errorMessage = 'Connection failed. Please check your credentials.';
        }
      });
    } catch (e) {
      setState(() {
        _errorMessage = 'Error: ${e.toString()}';
      });
    } finally {
      setState(() {
        _isTesting = false;
      });
    }
  }

  Future<void> _saveAndContinue() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _isLoading = true;
      _errorMessage = null;
    });

    try {
      final credentials = EbayCredentials(
        appId: _appIdController.text.trim(),
        certId: _certIdController.text.trim(),
        devId: _devIdController.text.trim().isEmpty ? null : _devIdController.text.trim(),
        ruName: _ruNameController.text.trim().isEmpty ? null : _ruNameController.text.trim(),
      );

      await CredentialsService.instance.saveCredentials(credentials);
      await CredentialsService.instance.saveEnvironment(_selectedEnvironment);

      widget.onSetupComplete();
    } catch (e) {
      setState(() {
        _errorMessage = 'Failed to save credentials: ${e.toString()}';
      });
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  @override
  void dispose() {
    _appIdController.dispose();
    _certIdController.dispose();
    _devIdController.dispose();
    _ruNameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 500),
            child: Card(
              elevation: 4,
              child: Padding(
                padding: const EdgeInsets.all(32),
                child: Form(
                  key: _formKey,
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      // Header
                      Icon(
                        Icons.settings_outlined,
                        size: 48,
                        color: Theme.of(context).primaryColor,
                      ),
                      const SizedBox(height: 16),
                      Text(
                        'eBay Postage Helper',
                        style: Theme.of(context).textTheme.headlineMedium,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Configure your eBay API credentials to get started',
                        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                          color: Colors.grey[600],
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 32),

                      // Environment selector
                      Text(
                        'Environment',
                        style: Theme.of(context).textTheme.titleSmall,
                      ),
                      const SizedBox(height: 8),
                      SegmentedButton<EbayEnvironment>(
                        segments: const [
                          ButtonSegment(
                            value: EbayEnvironment.sandbox,
                            label: Text('Sandbox'),
                            icon: Icon(Icons.science_outlined),
                          ),
                          ButtonSegment(
                            value: EbayEnvironment.production,
                            label: Text('Production'),
                            icon: Icon(Icons.public),
                          ),
                        ],
                        selected: {_selectedEnvironment},
                        onSelectionChanged: (Set<EbayEnvironment> selection) {
                          setState(() {
                            _selectedEnvironment = selection.first;
                          });
                        },
                      ),
                      const SizedBox(height: 24),

                      // App ID (Client ID)
                      TextFormField(
                        controller: _appIdController,
                        decoration: const InputDecoration(
                          labelText: 'App ID (Client ID)',
                          hintText: 'Enter your eBay App ID',
                          prefixIcon: Icon(Icons.key),
                          border: OutlineInputBorder(),
                        ),
                        validator: (value) {
                          if (value == null || value.trim().isEmpty) {
                            return 'App ID is required';
                          }
                          return null;
                        },
                      ),
                      const SizedBox(height: 16),

                      // Cert ID (Client Secret)
                      TextFormField(
                        controller: _certIdController,
                        obscureText: _obscureCertId,
                        decoration: InputDecoration(
                          labelText: 'Cert ID (Client Secret)',
                          hintText: 'Enter your eBay Cert ID',
                          prefixIcon: const Icon(Icons.lock),
                          border: const OutlineInputBorder(),
                          suffixIcon: IconButton(
                            icon: Icon(
                              _obscureCertId ? Icons.visibility : Icons.visibility_off,
                            ),
                            onPressed: () {
                              setState(() {
                                _obscureCertId = !_obscureCertId;
                              });
                            },
                          ),
                        ),
                        validator: (value) {
                          if (value == null || value.trim().isEmpty) {
                            return 'Cert ID is required';
                          }
                          return null;
                        },
                      ),
                      const SizedBox(height: 16),

                      // Dev ID (Optional)
                      TextFormField(
                        controller: _devIdController,
                        decoration: const InputDecoration(
                          labelText: 'Dev ID (Optional)',
                          hintText: 'Enter your eBay Developer ID',
                          prefixIcon: Icon(Icons.developer_mode),
                          border: OutlineInputBorder(),
                        ),
                      ),
                      const SizedBox(height: 16),

                      // RuName (Optional)
                      TextFormField(
                        controller: _ruNameController,
                        decoration: const InputDecoration(
                          labelText: 'RuName (Optional)',
                          hintText: 'Redirect URL name for OAuth',
                          prefixIcon: Icon(Icons.link),
                          border: OutlineInputBorder(),
                        ),
                      ),
                      const SizedBox(height: 24),

                      // Error/Success messages
                      if (_errorMessage != null)
                        Container(
                          padding: const EdgeInsets.all(12),
                          decoration: BoxDecoration(
                            color: Colors.red[50],
                            borderRadius: BorderRadius.circular(8),
                            border: Border.all(color: Colors.red[200]!),
                          ),
                          child: Row(
                            children: [
                              Icon(Icons.error_outline, color: Colors.red[700]),
                              const SizedBox(width: 12),
                              Expanded(
                                child: Text(
                                  _errorMessage!,
                                  style: TextStyle(color: Colors.red[700]),
                                ),
                              ),
                            ],
                          ),
                        ),

                      if (_successMessage != null)
                        Container(
                          padding: const EdgeInsets.all(12),
                          decoration: BoxDecoration(
                            color: Colors.green[50],
                            borderRadius: BorderRadius.circular(8),
                            border: Border.all(color: Colors.green[200]!),
                          ),
                          child: Row(
                            children: [
                              Icon(Icons.check_circle_outline, color: Colors.green[700]),
                              const SizedBox(width: 12),
                              Expanded(
                                child: Text(
                                  _successMessage!,
                                  style: TextStyle(color: Colors.green[700]),
                                ),
                              ),
                            ],
                          ),
                        ),

                      if (_errorMessage != null || _successMessage != null)
                        const SizedBox(height: 16),

                      // Action buttons
                      Row(
                        children: [
                          Expanded(
                            child: OutlinedButton.icon(
                              onPressed: _isTesting || _isLoading ? null : _testConnection,
                              icon: _isTesting
                                  ? const SizedBox(
                                      width: 16,
                                      height: 16,
                                      child: CircularProgressIndicator(strokeWidth: 2),
                                    )
                                  : const Icon(Icons.wifi_tethering),
                              label: Text(_isTesting ? 'Testing...' : 'Test Connection'),
                            ),
                          ),
                          const SizedBox(width: 16),
                          Expanded(
                            child: FilledButton.icon(
                              onPressed: _isLoading || _isTesting ? null : _saveAndContinue,
                              icon: _isLoading
                                  ? const SizedBox(
                                      width: 16,
                                      height: 16,
                                      child: CircularProgressIndicator(
                                        strokeWidth: 2,
                                        color: Colors.white,
                                      ),
                                    )
                                  : const Icon(Icons.arrow_forward),
                              label: Text(_isLoading ? 'Saving...' : 'Continue'),
                            ),
                          ),
                        ],
                      ),

                      const SizedBox(height: 24),

                      // Help text
                      Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: Colors.blue[50],
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              children: [
                                Icon(Icons.info_outline, size: 16, color: Colors.blue[700]),
                                const SizedBox(width: 8),
                                Text(
                                  'Where to find your credentials',
                                  style: TextStyle(
                                    fontWeight: FontWeight.bold,
                                    color: Colors.blue[700],
                                  ),
                                ),
                              ],
                            ),
                            const SizedBox(height: 8),
                            Text(
                              '1. Go to developer.ebay.com\n'
                              '2. Sign in and go to "My Account"\n'
                              '3. Select "Application Keys"\n'
                              '4. Copy your App ID and Cert ID',
                              style: TextStyle(
                                fontSize: 12,
                                color: Colors.blue[900],
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
