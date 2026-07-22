import 'dart:convert';
import 'dart:async';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/io.dart';

/// Simple API client for Proxi Messenger server.
///
/// All methods are static so they can be called from anywhere without
/// passing around an instance. Set [token] after login/signup.
class ApiClient {
  static String baseUrl = 'http://localhost:9999';
  static String? token;

  // ── Generic helpers ─────────────────────────────────────────────

  static Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (token != null) 'Authorization': 'Bearer $token',
      };

  /// POST [path] with JSON [body]. Returns decoded response map.
  static Future<Map<String, dynamic>> post(
      String path, Map<String, dynamic> body) async {
    final uri = Uri.parse('$baseUrl$path');
    final response = await http
        .post(uri, headers: _headers, body: jsonEncode(body))
        .timeout(const Duration(seconds: 15));
    if (response.statusCode >= 400) {
      final err = jsonDecode(response.body);
      throw Exception(err['error'] ?? 'Request failed (${response.statusCode})');
    }
    return jsonDecode(response.body) as Map<String, dynamic>;
  }

  /// GET [path] with auth header. Returns decoded response as a list.
  static Future<List<dynamic>> get(String path) async {
    final uri = Uri.parse('$baseUrl$path');
    final response = await http
        .get(uri, headers: _headers)
        .timeout(const Duration(seconds: 15));
    if (response.statusCode >= 400) {
      final err = jsonDecode(response.body);
      throw Exception(err['error'] ?? 'Request failed (${response.statusCode})');
    }
    final decoded = jsonDecode(response.body);
    if (decoded is List) return decoded;
    if (decoded is Map<String, dynamic>) {
      // Unwrap common wrapper keys
      return decoded['channels'] as List<dynamic>? ??
          decoded['messages'] as List<dynamic>? ??
          decoded['stories'] as List<dynamic>? ??
          decoded['data'] as List<dynamic>? ??
          [decoded];
    }
    return [];
  }

  // ── Auth ────────────────────────────────────────────────────────

  /// Login with a Nostr npub key. Stores the returned JWT in [token].
  static Future<void> login(String npub) async {
    final resp = await post('/api/auth/login', {'npub': npub});
    token = resp['token'] as String?;
  }

  /// Sign up with a Nostr npub key and a display [username].
  static Future<void> signup(String npub, String username) async {
    final resp = await post('/api/auth/signup', {
      'npub': npub,
      'username': username,
    });
    token = resp['token'] as String?;
  }

  // ── Messages ────────────────────────────────────────────────────

  /// Fetch messages for a [channel] (or chat).
  static Future<List<dynamic>> getMessages(String channel) async {
    return get('/api/messages?channel=$channel');
  }

  /// Send a text message to recipient [to].
  static Future<void> sendMessage(String to, String text) async {
    await post('/api/message', {'to': to, 'text': text});
  }

  // ── Channels ────────────────────────────────────────────────────

  /// Get the list of subscribed channels.
  static Future<List<dynamic>> getChannels() async {
    return get('/api/channels');
  }

  // ── Stories ─────────────────────────────────────────────────────

  /// Get available stories.
  static Future<List<dynamic>> getStories() async {
    return get('/api/stories');
  }

  // ── Profile ─────────────────────────────────────────────────────

  /// Get the current user's identity / profile.
  static Future<Map<String, dynamic>> getProfile() async {
    final uri = Uri.parse('$baseUrl/api/identity');
    final response = await http
        .get(uri, headers: _headers)
        .timeout(const Duration(seconds: 15));
    if (response.statusCode >= 400) {
      throw Exception('Failed to load profile (${response.statusCode})');
    }
    return jsonDecode(response.body) as Map<String, dynamic>;
  }

  // ── WebSocket ───────────────────────────────────────────────────

  /// Open a WebSocket connection with the current auth [token] as a
  /// query parameter. Returns `null` when there is no token.
  static IOWebSocketChannel? connectWS() {
    if (token == null) return null;
    final wsUrl = baseUrl.replaceFirst('http', 'ws');
    final uri = Uri.parse('$wsUrl/ws?token=$token');
    return IOWebSocketChannel.connect(uri);
  }
}
