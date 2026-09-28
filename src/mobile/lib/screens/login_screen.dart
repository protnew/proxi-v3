import 'package:flutter/material.dart';
import '../services/api_client.dart';
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
  final _npubController = TextEditingController();
  final _usernameController = TextEditingController();
  final _e2eService = E2EService();
  final _storageService = StorageService();

  bool _isLoading = false;
  bool _isLoginMode = true; // true = login, false = signup
  String? _error;

  @override
  void initState() {
    super.initState();
    _checkExistingIdentity();
  }

  Future<void> _checkExistingIdentity() async {
    await _storageService.init();
    final identity = _storageService.getIdentity();
    final token = _storageService.getAuthToken();
    if (identity != null && token != null) {
      ApiClient.token = token;
      _navigateToMain();
    }
  }

  Future<void> _submit() async {
    final npub = _npubController.text.trim();
    if (npub.isEmpty) {
      setState(() => _error = 'Please enter your npub key');
      return;
    }

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      if (_isLoginMode) {
        // Login flow
        await ApiClient.login(npub);
      } else {
        // Signup flow
        final username = _usernameController.text.trim();
        if (username.isEmpty || username.length < 3) {
          setState(() {
            _error = 'Username must be at least 3 characters';
            _isLoading = false;
          });
          return;
        }
        await ApiClient.signup(npub, username);
      }

      // Save token locally
      if (ApiClient.token != null) {
        await _storageService.saveAuthToken(ApiClient.token!);
      }

      // Save a basic identity record
      await _storageService.saveIdentity({
        'npub': npub,
        'username': _isLoginMode ? '' : _usernameController.text.trim(),
      });

      _navigateToMain();
    } catch (e) {
      setState(() {
        _error = '${_isLoginMode ? "Login" : "Signup"} failed: $e';
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
    _npubController.dispose();
    _usernameController.dispose();
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

                // Mode toggle (Login / Signup)
                Container(
                  decoration: BoxDecoration(
                    color: ProxiTheme.surfaceVariant,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: GestureDetector(
                          onTap: () => setState(() => _isLoginMode = true),
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            decoration: BoxDecoration(
                              color: _isLoginMode ? ProxiTheme.primary : Colors.transparent,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: Text(
                              'Login',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: _isLoginMode ? Colors.white : ProxiTheme.muted,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                        ),
                      ),
                      Expanded(
                        child: GestureDetector(
                          onTap: () => setState(() => _isLoginMode = false),
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            decoration: BoxDecoration(
                              color: !_isLoginMode ? ProxiTheme.primary : Colors.transparent,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: Text(
                              'Sign Up',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: !_isLoginMode ? Colors.white : ProxiTheme.muted,
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

                // npub field
                TextField(
                  controller: _npubController,
                  style: const TextStyle(color: ProxiTheme.onBackground),
                  decoration: const InputDecoration(
                    hintText: 'Enter your npub key',
                    prefixIcon: Icon(Icons.key, color: ProxiTheme.muted),
                  ),
                  onSubmitted: (_) => _submit(),
                ),
                const SizedBox(height: 16),

                // Username field (signup only)
                if (!_isLoginMode) ...[
                  TextField(
                    controller: _usernameController,
                    style: const TextStyle(color: ProxiTheme.onBackground),
                    decoration: const InputDecoration(
                      hintText: 'Choose a username',
                      prefixIcon: Icon(Icons.person_outline, color: ProxiTheme.muted),
                    ),
                    onSubmitted: (_) => _submit(),
                  ),
                  const SizedBox(height: 16),
                ],

                Text(
                  _isLoginMode
                      ? 'Enter your Nostr public key (npub) to log in.'
                      : 'A new encryption key pair will be generated for you.',
                  style: Theme.of(context).textTheme.labelSmall,
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 32),

                // Submit button
                SizedBox(
                  width: double.infinity,
                  height: 50,
                  child: ElevatedButton(
                    onPressed: _isLoading ? null : _submit,
                    child: _isLoading
                        ? const SizedBox(
                            width: 24,
                            height: 24,
                            child: CircularProgressIndicator(
                              color: Colors.white,
                              strokeWidth: 2,
                            ),
                          )
                        : Text(_isLoginMode ? 'Login' : 'Create Account'),
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
