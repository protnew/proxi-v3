import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/e2e_service.dart';
import '../services/storage_service.dart';
import '../services/theme.dart';
import '../widgets/contact_avatar.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  final _apiService = ApiService();
  final _e2eService = E2EService();
  final _storageService = StorageService();

  final _displayNameController = TextEditingController();
  final _statusController = TextEditingController();

  Map<String, dynamic>? _identity;
  bool _isLoading = true;
  bool _isEditing = false;

  @override
  void initState() {
    super.initState();
    _loadProfile();
  }

  Future<void> _loadProfile() async {
    await _storageService.init();
    final identity = _storageService.getIdentity();
    final token = _storageService.getAuthToken();
    if (token != null) _apiService.setAuthToken(token);

    if (identity != null) {
      _displayNameController.text = identity['display_name'] ?? '';
      setState(() {
        _identity = identity;
        _isLoading = false;
      });
    } else {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _saveProfile() async {
    setState(() => _isEditing = false);
    try {
      await _apiService.updateProfile(
        displayName: _displayNameController.text.trim(),
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Profile updated')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error: $e')),
        );
      }
    }
  }

  void _copyToClipboard(String text) {
    // TODO: use Clipboard.setData
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('Copied to clipboard')),
    );
  }

  @override
  void dispose() {
    _displayNameController.dispose();
    _statusController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Profile'),
        actions: [
          IconButton(
            icon: Icon(_isEditing ? Icons.check : Icons.edit),
            onPressed: () {
              if (_isEditing) {
                _saveProfile();
              } else {
                setState(() => _isEditing = true);
              }
            },
          ),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator(color: ProxiTheme.primary))
          : SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: Column(
                children: [
                  // Avatar
                  GestureDetector(
                    onTap: () {
                      // TODO: Change avatar
                    },
                    child: Stack(
                      children: [
                        ContactAvatar(
                          name: _identity?['username'] ?? 'User',
                          radius: 48,
                        ),
                        Positioned(
                          right: 0,
                          bottom: 0,
                          child: Container(
                            width: 32,
                            height: 32,
                            decoration: BoxDecoration(
                              color: ProxiTheme.primary,
                              shape: BoxShape.circle,
                              border: Border.all(color: ProxiTheme.surface, width: 2),
                            ),
                            child: const Icon(Icons.camera_alt, size: 16, color: Colors.white),
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),

                  // Username
                  if (!_isEditing) ...[
                    Text(
                      _identity?['display_name'] ?? _identity?['username'] ?? 'Unknown',
                      style: Theme.of(context).textTheme.headlineMedium,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '@${_identity?['username'] ?? ''}',
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: ProxiTheme.primary),
                    ),
                  ] else ...[
                    TextField(
                      controller: _displayNameController,
                      style: const TextStyle(color: ProxiTheme.onBackground),
                      decoration: const InputDecoration(
                        labelText: 'Display Name',
                        labelStyle: TextStyle(color: ProxiTheme.muted),
                      ),
                    ),
                    const SizedBox(height: 16),
                    TextField(
                      controller: _statusController,
                      style: const TextStyle(color: ProxiTheme.onBackground),
                      decoration: const InputDecoration(
                        labelText: 'Status',
                        labelStyle: TextStyle(color: ProxiTheme.muted),
                      ),
                    ),
                  ],

                  const SizedBox(height: 32),
                  const Divider(color: ProxiTheme.divider),
                  const SizedBox(height: 16),

                  // Identity ID
                  _InfoTile(
                    icon: Icons.fingerprint,
                    title: 'Identity ID',
                    value: _identity?['id'] ?? 'N/A',
                    onTap: () => _copyToClipboard(_identity?['id'] ?? ''),
                  ),

                  // Public Key
                  _InfoTile(
                    icon: Icons.vpn_key,
                    title: 'Public Key',
                    value: _identity?['public_key'] ?? 'N/A',
                    onTap: () => _copyToClipboard(_identity?['public_key'] ?? ''),
                    mono: true,
                  ),

                  const SizedBox(height: 16),
                  const Divider(color: ProxiTheme.divider),
                  const SizedBox(height: 16),

                  // E2E Info
                  Container(
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: ProxiTheme.secondary.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: ProxiTheme.secondary.withValues(alpha: 0.3)),
                    ),
                    child: Row(
                      children: [
                        const Icon(Icons.shield, color: ProxiTheme.secondary, size: 28),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                'End-to-End Encrypted',
                                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                                      color: ProxiTheme.secondary,
                                    ),
                              ),
                              const SizedBox(height: 4),
                              Text(
                                'Your messages are encrypted with X25519 + AES-256-GCM',
                                style: Theme.of(context).textTheme.labelSmall,
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),

                  const SizedBox(height: 24),

                  // Danger zone
                  Text(
                    'DANGER ZONE',
                    style: Theme.of(context).textTheme.labelLarge?.copyWith(color: ProxiTheme.danger),
                  ),
                  const SizedBox(height: 12),
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton.icon(
                      onPressed: () async {
                        final confirm = await showDialog<bool>(
                          context: context,
                          builder: (ctx) => AlertDialog(
                            title: const Text('Delete Identity?'),
                            content: const Text(
                              'This will permanently delete your identity and all local data. '
                              'Make sure you have backed up your Identity ID.',
                            ),
                            actions: [
                              TextButton(
                                onPressed: () => Navigator.pop(ctx, false),
                                child: const Text('Cancel'),
                              ),
                              ElevatedButton(
                                style: ElevatedButton.styleFrom(backgroundColor: ProxiTheme.danger),
                                onPressed: () => Navigator.pop(ctx, true),
                                child: const Text('Delete'),
                              ),
                            ],
                          ),
                        );
                        if (confirm == true) {
                          await _storageService.clearAll();
                          if (mounted) {
                            Navigator.of(context).pushNamedAndRemoveUntil('/', (_) => false);
                          }
                        }
                      },
                      icon: const Icon(Icons.delete_forever, color: ProxiTheme.danger),
                      label: const Text('Delete Identity', style: TextStyle(color: ProxiTheme.danger)),
                      style: OutlinedButton.styleFrom(
                        side: const BorderSide(color: ProxiTheme.danger),
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                      ),
                    ),
                  ),
                ],
              ),
            ),
    );
  }
}

class _InfoTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String value;
  final VoidCallback onTap;
  final bool mono;

  const _InfoTile({
    required this.icon,
    required this.title,
    required this.value,
    required this.onTap,
    this.mono = false,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      onTap: onTap,
      contentPadding: EdgeInsets.zero,
      leading: Icon(icon, color: ProxiTheme.primary, size: 22),
      title: Text(title, style: Theme.of(context).textTheme.labelSmall),
      subtitle: Text(
        value.length > 40 ? '${value.substring(0, 40)}...' : value,
        style: TextStyle(
          color: ProxiTheme.onBackground,
          fontSize: 13,
          fontFamily: mono ? 'RobotoMono' : null,
        ),
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      ),
      trailing: const Icon(Icons.copy, size: 18, color: ProxiTheme.muted),
    );
  }
}
