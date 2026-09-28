import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';
// Note: hive_flutter will be used via generated adapters in production.
// For now we use SharedPreferences for simple key-value + JSON blobs.

class StorageService {
  static const _keyIdentity = 'proxi_identity';
  static const _keyAuthToken = 'proxi_auth_token';
  static const _keySettings = 'proxi_settings';
  static const _keyChatCache = 'proxi_chat_cache_';
  static const _keyMessageCache = 'proxi_msg_cache_';

  SharedPreferences? _prefs;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
  }

  SharedPreferences get _p {
    assert(_prefs != null, 'StorageService not initialized. Call init() first.');
    return _prefs!;
  }

  // ── Identity ──────────────────────────────────────────────────────

  Future<void> saveIdentity(Map<String, dynamic> identity) async {
    await _p.setString(_keyIdentity, jsonEncode(identity));
  }

  Map<String, dynamic>? getIdentity() {
    final raw = _p.getString(_keyIdentity);
    if (raw == null) return null;
    return jsonDecode(raw) as Map<String, dynamic>;
  }

  Future<void> clearIdentity() async {
    await _p.remove(_keyIdentity);
  }

  // ── Auth Token ────────────────────────────────────────────────────

  Future<void> saveAuthToken(String token) async {
    await _p.setString(_keyAuthToken, token);
  }

  String? getAuthToken() => _p.getString(_keyAuthToken);

  Future<void> clearAuthToken() async {
    await _p.remove(_keyAuthToken);
  }

  // ── Settings ──────────────────────────────────────────────────────

  Future<void> saveSetting(String key, dynamic value) async {
    final settings = getSettings();
    settings[key] = value;
    await _p.setString(_keySettings, jsonEncode(settings));
  }

  Map<String, dynamic> getSettings() {
    final raw = _p.getString(_keySettings);
    if (raw == null) return _defaultSettings();
    return jsonDecode(raw) as Map<String, dynamic>;
  }

  T getSetting<T>(String key, T defaultValue) {
    final settings = getSettings();
    return settings[key] as T? ?? defaultValue;
  }

  Map<String, dynamic> _defaultSettings() => {
        'notifications_enabled': true,
        'e2e_by_default': true,
        'send_with_enter': true,
        'font_size': 14.0,
        'language': 'ru',
        'theme': 'dark',
      };

  // ── Chat Cache ────────────────────────────────────────────────────

  Future<void> cacheChats(List<Map<String, dynamic>> chats) async {
    await _p.setString('$_keyChatCache\$all', jsonEncode(chats));
  }

  List<Map<String, dynamic>> getCachedChats() {
    final raw = _p.getString('$_keyChatCache\$all');
    if (raw == null) return [];
    return (jsonDecode(raw) as List).cast<Map<String, dynamic>>();
  }

  // ── Message Cache ─────────────────────────────────────────────────

  Future<void> cacheMessages(String chatId, List<Map<String, dynamic>> messages) async {
    await _p.setString('$_keyMessageCache$chatId', jsonEncode(messages));
  }

  List<Map<String, dynamic>> getCachedMessages(String chatId) {
    final raw = _p.getString('$_keyMessageCache$chatId');
    if (raw == null) return [];
    return (jsonDecode(raw) as List).cast<Map<String, dynamic>>();
  }

  // ── Clear All ─────────────────────────────────────────────────────

  Future<void> clearAll() async {
    await _p.clear();
  }
}
