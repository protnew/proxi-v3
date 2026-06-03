import 'package:flutter/material.dart';
import '../services/storage_service.dart';
import '../services/theme.dart';
import 'profile_screen.dart';
import '../screens/login_screen.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  final _storageService = StorageService();

  bool _notificationsEnabled = true;
  bool _e2eByDefault = true;
  bool _sendWithEnter = true;
  double _fontSize = 14.0;
  String _language = 'ru';

  @override
  void initState() {
    super.initState();
    _loadSettings();
  }

  Future<void> _loadSettings() async {
    await _storageService.init();
    setState(() {
      _notificationsEnabled = _storageService.getSetting('notifications_enabled', true);
      _e2eByDefault = _storageService.getSetting('e2e_by_default', true);
      _sendWithEnter = _storageService.getSetting('send_with_enter', true);
      _fontSize = _storageService.getSetting('font_size', 14.0);
      _language = _storageService.getSetting('language', 'ru');
    });
  }

  Future<void> _updateSetting(String key, dynamic value) async {
    await _storageService.saveSetting(key, value);
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // Profile section
        Card(
          child: ListTile(
            onTap: () {
              Navigator.push(
                context,
                MaterialPageRoute(builder: (_) => const ProfileScreen()),
              );
            },
            leading: const CircleAvatar(
              radius: 24,
              backgroundColor: ProxiTheme.primary,
              child: Icon(Icons.person, color: Colors.white, size: 28),
            ),
            title: const Text(
              'My Profile',
              style: TextStyle(color: ProxiTheme.onBackground, fontWeight: FontWeight.w600),
            ),
            subtitle: const Text('View and edit your profile & keys'),
            trailing: const Icon(Icons.chevron_right, color: ProxiTheme.muted),
          ),
        ),
        const SizedBox(height: 16),

        // Messaging section
        Text(
          'MESSAGING',
          style: Theme.of(context).textTheme.labelLarge,
        ),
        const SizedBox(height: 8),
        Card(
          child: Column(
            children: [
              SwitchListTile(
                title: const Text('E2E Encryption by default'),
                subtitle: const Text('Encrypt all new chats'),
                secondary: const Icon(Icons.lock, color: ProxiTheme.secondary),
                value: _e2eByDefault,
                activeColor: ProxiTheme.primary,
                onChanged: (v) {
                  setState(() => _e2eByDefault = v);
                  _updateSetting('e2e_by_default', v);
                },
              ),
              SwitchListTile(
                title: const Text('Send with Enter'),
                subtitle: const Text('Send message on Enter key'),
                secondary: const Icon(Icons.keyboard_return, color: ProxiTheme.muted),
                value: _sendWithEnter,
                activeColor: ProxiTheme.primary,
                onChanged: (v) {
                  setState(() => _sendWithEnter = v);
                  _updateSetting('send_with_enter', v);
                },
              ),
              ListTile(
                title: const Text('Font Size'),
                secondary: const Icon(Icons.text_fields, color: ProxiTheme.muted),
                subtitle: Slider(
                  value: _fontSize,
                  min: 12.0,
                  max: 20.0,
                  divisions: 8,
                  activeColor: ProxiTheme.primary,
                  label: _fontSize.round().toString(),
                  onChanged: (v) {
                    setState(() => _fontSize = v);
                    _updateSetting('font_size', v);
                  },
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // Notifications section
        Text(
          'NOTIFICATIONS',
          style: Theme.of(context).textTheme.labelLarge,
        ),
        const SizedBox(height: 8),
        Card(
          child: SwitchListTile(
            title: const Text('Notifications'),
            subtitle: const Text('Receive message notifications'),
            secondary: const Icon(Icons.notifications_outlined, color: ProxiTheme.muted),
            value: _notificationsEnabled,
            activeColor: ProxiTheme.primary,
            onChanged: (v) {
              setState(() => _notificationsEnabled = v);
              _updateSetting('notifications_enabled', v);
            },
          ),
        ),
        const SizedBox(height: 16),

        // Language section
        Text(
          'APPEARANCE',
          style: Theme.of(context).textTheme.labelLarge,
        ),
        const SizedBox(height: 8),
        Card(
          child: Column(
            children: [
              ListTile(
                title: const Text('Language'),
                secondary: const Icon(Icons.language, color: ProxiTheme.muted),
                trailing: Text(
                  _language == 'ru' ? 'Русский' : 'English',
                  style: const TextStyle(color: ProxiTheme.primary),
                ),
                onTap: () {
                  setState(() {
                    _language = _language == 'ru' ? 'en' : 'ru';
                  });
                  _updateSetting('language', _language);
                },
              ),
              ListTile(
                title: const Text('Theme'),
                subtitle: const Text('Dark'),
                secondary: const Icon(Icons.dark_mode, color: ProxiTheme.muted),
                trailing: const Icon(Icons.check_circle, color: ProxiTheme.primary, size: 20),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // Network section
        Text(
          'NETWORK',
          style: Theme.of(context).textTheme.labelLarge,
        ),
        const SizedBox(height: 8),
        Card(
          child: Column(
            children: [
              ListTile(
                title: const Text('Server URL'),
                subtitle: const Text(
                  'http://localhost:9999',
                  style: TextStyle(fontFamily: 'RobotoMono', fontSize: 13),
                ),
                leading: const Icon(Icons.dns, color: ProxiTheme.muted),
                trailing: const Icon(Icons.edit, size: 18, color: ProxiTheme.muted),
                onTap: () {
                  // TODO: Edit server URL
                },
              ),
              ListTile(
                title: const Text('Connection Status'),
                subtitle: Row(
                  children: [
                    Container(
                      width: 8,
                      height: 8,
                      decoration: const BoxDecoration(
                        color: ProxiTheme.secondary,
                        shape: BoxShape.circle,
                      ),
                    ),
                    const SizedBox(width: 6),
                    const Text('Connected'),
                  ],
                ),
                leading: const Icon(Icons.cloud_done, color: ProxiTheme.secondary),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // About section
        Text(
          'ABOUT',
          style: Theme.of(context).textTheme.labelLarge,
        ),
        const SizedBox(height: 8),
        Card(
          child: Column(
            children: [
              const ListTile(
                title: Text('Proxi Messenger'),
                subtitle: Text('Version 1.0.0'),
                leading: Icon(Icons.info_outline, color: ProxiTheme.muted),
              ),
              ListTile(
                title: const Text('Source Code'),
                subtitle: const Text('github.com/proxi-messenger'),
                leading: const Icon(Icons.code, color: ProxiTheme.muted),
                onTap: () {
                  // TODO: Open URL
                },
              ),
              ListTile(
                title: const Text('Privacy Policy'),
                leading: const Icon(Icons.privacy_tip, color: ProxiTheme.muted),
                onTap: () {
                  // TODO: Open privacy policy
                },
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),

        // Logout
        SizedBox(
          width: double.infinity,
          child: OutlinedButton.icon(
            onPressed: () async {
              final confirm = await showDialog<bool>(
                context: context,
                builder: (ctx) => AlertDialog(
                  title: const Text('Logout?'),
                  content: const Text('Your identity data will remain on this device.'),
                  actions: [
                    TextButton(
                      onPressed: () => Navigator.pop(ctx, false),
                      child: const Text('Cancel'),
                    ),
                    ElevatedButton(
                      onPressed: () => Navigator.pop(ctx, true),
                      child: const Text('Logout'),
                    ),
                  ],
                ),
              );
              if (confirm == true) {
                await _storageService.clearAuthToken();
                if (mounted) {
                  Navigator.of(context).pushNamedAndRemoveUntil('/', (_) => false);
                }
              }
            },
            icon: const Icon(Icons.logout, color: ProxiTheme.danger),
            label: const Text('Logout', style: TextStyle(color: ProxiTheme.danger)),
            style: OutlinedButton.styleFrom(
              side: const BorderSide(color: ProxiTheme.danger),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            ),
          ),
        ),
        const SizedBox(height: 32),
      ],
    );
  }
}
