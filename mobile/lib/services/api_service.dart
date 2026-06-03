import 'dart:convert';
import 'dart:async';
import 'package:http/http.dart' as http;

class ApiException implements Exception {
  final int? statusCode;
  final String message;
  ApiException(this.message, {this.statusCode});
  @override
  String toString() => 'ApiException: $message (${statusCode ?? "N/A"})';
}

class ApiService {
  static const String _baseUrl = 'http://localhost:9999/api/v1';

  final http.Client _client;
  String? _authToken;

  ApiService({http.Client? client}) : _client = client ?? http.Client();

  void setAuthToken(String token) {
    _authToken = token;
  }

  void clearAuthToken() {
    _authToken = null;
  }

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (_authToken != null) 'Authorization': 'Bearer $_authToken',
      };

  // ── Identity / Auth ──────────────────────────────────────────────

  Future<Map<String, dynamic>> createIdentity(String username, String publicKey) async {
    return _post('/identity/create', {
      'username': username,
      'public_key': publicKey,
    });
  }

  Future<Map<String, dynamic>> restoreIdentity(String identityId) async {
    return _post('/identity/restore', {
      'identity_id': identityId,
    });
  }

  Future<Map<String, dynamic>> getIdentity() async {
    return _get('/identity/me');
  }

  Future<Map<String, dynamic>> updateProfile({
    String? displayName,
    String? avatarUrl,
    String? status,
  }) async {
    final body = <String, dynamic>{};
    if (displayName != null) body['display_name'] = displayName;
    if (avatarUrl != null) body['avatar_url'] = avatarUrl;
    if (status != null) body['status'] = status;
    return _patch('/identity/me', body);
  }

  // ── Chats ────────────────────────────────────────────────────────

  Future<List<dynamic>> getChats() async {
    final resp = await _get('/chats');
    return resp['chats'] as List<dynamic>? ?? [];
  }

  Future<Map<String, dynamic>> createChat(String peerId, {bool e2e = false}) async {
    return _post('/chats/create', {
      'peer_id': peerId,
      'e2e': e2e,
    });
  }

  Future<Map<String, dynamic>> getChat(String chatId) async {
    return _get('/chats/$chatId');
  }

  Future<void> deleteChat(String chatId) async {
    await _delete('/chats/$chatId');
  }

  // ── Messages ─────────────────────────────────────────────────────

  Future<List<dynamic>> getMessages(String chatId, {int limit = 50, String? before}) async {
    final query = <String, String>{'limit': '$limit'};
    if (before != null) query['before'] = before;
    final resp = await _get('/chats/$chatId/messages?$limit');
    return resp['messages'] as List<dynamic>? ?? [];
  }

  Future<Map<String, dynamic>> sendMessage(
    String chatId,
    String text, {
    String? replyTo,
    Map<String, dynamic>? metadata,
  }) async {
    final body = <String, dynamic>{
      'text': text,
      if (replyTo != null) 'reply_to': replyTo,
      if (metadata != null) 'metadata': metadata,
    };
    return _post('/chats/$chatId/messages', body);
  }

  Future<void> deleteMessage(String chatId, String messageId) async {
    await _delete('/chats/$chatId/messages/$messageId');
  }

  Future<void> markRead(String chatId) async {
    await _post('/chats/$chatId/read', {});
  }

  // ── Channels ─────────────────────────────────────────────────────

  Future<List<dynamic>> getChannels() async {
    final resp = await _get('/channels');
    return resp['channels'] as List<dynamic>? ?? [];
  }

  Future<Map<String, dynamic>> createChannel(String name, {String? description}) async {
    return _post('/channels/create', {
      'name': name,
      if (description != null) 'description': description,
    });
  }

  Future<Map<String, dynamic>> getChannel(String channelId) async {
    return _get('/channels/$channelId');
  }

  Future<void> subscribe(String channelId) async {
    await _post('/channels/$channelId/subscribe', {});
  }

  Future<void> unsubscribe(String channelId) async {
    await _post('/channels/$channelId/unsubscribe', {});
  }

  Future<Map<String, dynamic>> publishToChannel(
    String channelId,
    String text, {
    Map<String, dynamic>? metadata,
  }) async {
    return _post('/channels/$channelId/publish', {
      'text': text,
      if (metadata != null) 'metadata': metadata,
    });
  }

  // ── Search ───────────────────────────────────────────────────────

  Future<List<dynamic>> searchUsers(String query) async {
    final resp = await _get('/users/search?q=${Uri.encodeComponent(query)}');
    return resp['users'] as List<dynamic>? ?? [];
  }

  // ── HTTP helpers ─────────────────────────────────────────────────

  Future<Map<String, dynamic>> _get(String path) async {
    final uri = Uri.parse('$_baseUrl$path');
    final response = await _client.get(uri, headers: _headers).timeout(const Duration(seconds: 15));
    return _handle(response);
  }

  Future<Map<String, dynamic>> _post(String path, Map<String, dynamic> body) async {
    final uri = Uri.parse('$_baseUrl$path');
    final response = await _client
        .post(uri, headers: _headers, body: jsonEncode(body))
        .timeout(const Duration(seconds: 15));
    return _handle(response);
  }

  Future<Map<String, dynamic>> _patch(String path, Map<String, dynamic> body) async {
    final uri = Uri.parse('$_baseUrl$path');
    final response = await _client
        .patch(uri, headers: _headers, body: jsonEncode(body))
        .timeout(const Duration(seconds: 15));
    return _handle(response);
  }

  Future<void> _delete(String path) async {
    final uri = Uri.parse('$_baseUrl$path');
    final response = await _client.delete(uri, headers: _headers).timeout(const Duration(seconds: 15));
    _handle(response);
  }

  Map<String, dynamic> _handle(http.Response response) {
    final body = jsonDecode(response.body) as Map<String, dynamic>;
    if (response.statusCode >= 400) {
      throw ApiException(body['error'] as String? ?? 'Unknown error', statusCode: response.statusCode);
    }
    return body;
  }
}
