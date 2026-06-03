import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/e2e_service.dart';
import '../services/storage_service.dart';
import '../services/theme.dart';
import 'chat_list_screen.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _usernameController = TextEditingController();
  final _restoreIdController = TextEditingController();
  final _apiService = ApiService();
  final _e2eService = E2EService();
  final _storageService = StorageService();

  bool _isLoading = false;
  bool _isRestoreMode = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _checkExistingIdentity();
  }

  Future<void> _checkExistingIdentity() async {
    await _storageService.init();
    final identity = _storageService.getIdentity();
    if (identity != null) {
      _navigateToMain();
    }
  }

  Future<void> _createIdentity() async {
    final username = _usernameController.text.trim();
    if (username.isEmpty || username.length < 3) {
      setState(() => _error = 'Username must be at least 3 characters');
      return;
    }

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      // Generate E2E key pair
      final keyPair = _e2eService.generateKeyPair();

      // Register on server
      final response = await _apiService.createIdentity(username, keyPair['public_key']!);

      final identity = {
        'id': response['identity_id'] ?? response['id'],
        'username': username,
        'public_key': keyPair['public_key'],
        'private_key_encrypted': keyPair['private_key'],
      };

      // Save locally
      await _storageService.saveIdentity(identity);
      if (response['token'] != null) {
        await _storageService.saveAuthToken(response['token'] as String);
      }

      _navigateToMain();
    } catch (e) {
      setState(() {
        _error = 'Failed to create identity: $e';
        _isLoading = false;
      });
    }
  }

  Future<void> _restoreIdentity() async {
    final identityId = _restoreIdController.text.trim();
    if (identityId.isEmpty) {
      setState(() => _error = 'Enter your Identity ID');
      return;
    }

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final response = await _apiService.restoreIdentity(identityId);

      final identity = {
        'id': identityId,
        'username': response['username'] ?? 'user',
        'public_key': response['public_key'],
        'private_key_encrypted': response['private_key_encrypted'],
      };

      await _storageService.saveIdentity(identity);
      if (response['token'] != null) {
        await _storageService.saveAuthToken(response['token'] as String);
      }

      _navigateToMain();
    } catch (e) {
      setState(() {
        _error = 'Failed to restore: $e';
        _isLoading = false;
      });
    }
  }

  void _navigateToMain() {
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const ChatListScreen()),
    );
  }

  @override
  void dispose() {
    _usernameController.dispose();
    _restoreIdController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                // Logo / Title
                Container(
                  width: 80,
                  height: 80,
                  decoration: BoxDecoration(
                    color: ProxiTheme.primary.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(24),
                  ),
                  child: const Icon(
                    Icons.shield_outlined,
                    color: ProxiTheme.primary,
                    size: 44,
                  ),
                ),
                const SizedBox(height: 24),
                Text(
                  'Proxi Messenger',
                  style: Theme.of(context).textTheme.headlineLarge?.copyWith(
                        color: ProxiTheme.primary,
                      ),
                ),
                const SizedBox(height: 8),
                Text(
                  'Decentralized • E2E Encrypted',
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: ProxiTheme.muted,
                      ),
                ),
                const SizedBox(height: 48),

                // Mode toggle
                Container(
                  decoration: BoxDecoration(
                    color: ProxiTheme.surfaceVariant,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: GestureDetector(
                          onTap: () => setState(() => _isRestoreMode = false),
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            decoration: BoxDecoration(
                              color: !_isRestoreMode ? ProxiTheme.primary : Colors.transparent,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: Text(
                              'Create',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: !_isRestoreMode ? Colors.white : ProxiTheme.muted,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                        ),
                      ),
                      Expanded(
                        child: GestureDetector(
                          onTap: () => setState(() => _isRestoreMode = true),
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            decoration: BoxDecoration(
                              color: _isRestoreMode ? ProxiTheme.primary : Colors.transparent,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: Text(
                              'Restore',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: _isRestoreMode ? Colors.white : ProxiTheme.muted,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 24),

                // Error message
                if (_error != null)
                  Container(
                    margin: const EdgeInsets.only(bottom: 16),
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: ProxiTheme.danger.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      children: [
                        const Icon(Icons.error_outline, color: ProxiTheme.danger, size: 20),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            _error!,
                            style: const TextStyle(color: ProxiTheme.danger, fontSize: 13),
                          ),
                        ),
                      ],
                    ),
                  ),

                if (!_isRestoreMode) ...[
                  // Create identity form
                  TextField(
                    controller: _usernameController,
                    style: const TextStyle(color: ProxiTheme.onBackground),
                    decoration: const InputDecoration(
                      hintText: 'Choose a username',
                      prefixIcon: Icon(Icons.person_outline, color: ProxiTheme.muted),
                    ),
                    onSubmitted: (_) => _createIdentity(),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    'A new encryption key pair will be generated for you.',
                    style: Theme.of(context).textTheme.labelSmall,
                    textAlign: TextAlign.center,
                  ),
                ] else ...[
                  // Restore identity form
                  TextField(
                    controller: _restoreIdController,
                    style: const TextStyle(color: ProxiTheme.onBackground),
                    decoration: const InputDecoration(
                      hintText: 'Enter your Identity ID',
                      prefixIcon: Icon(Icons.key, color: ProxiTheme.muted),
                    ),
                    onSubmitted: (_) => _restoreIdentity(),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    'Enter the Identity ID you saved during registration.',
                    style: Theme.of(context).textTheme.labelSmall,
                    textAlign: TextAlign.center,
                  ),
                ],

                const SizedBox(height: 32),

                // Submit button
                SizedBox(
                  width: double.infinity,
                  height: 50,
                  child: ElevatedButton(
                    onPressed: _isLoading
                        ? null
                        : (_isRestoreMode ? _restoreIdentity : _createIdentity),
                    child: _isLoading
                        ? const SizedBox(
                            width: 24,
                            height: 24,
                            child: CircularProgressIndicator(
                              color: Colors.white,
                              strokeWidth: 2,
                            ),
                          )
                        : Text(_isRestoreMode ? 'Restore Identity' : 'Create Identity'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
