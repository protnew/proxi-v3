import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';

/// Proxi Messenger SDK client — REST + WebSocket + binary frames.
class ProxiClient {
  final String baseUrl;
  WebSocketChannel? _ws;
  final _messageController = StreamController<Map<String, dynamic>>.broadcast();
  bool _connected = false;

  ProxiClient({this.baseUrl = 'http://localhost:9999'});

  bool get isConnected => _connected;

  /// Stream of incoming messages (JSON decoded).
  Stream<Map<String, dynamic>> get messages => _messageController.stream;

  String get _wsUrl =>
      baseUrl.replaceFirst('http://', 'ws://').replaceFirst('https://', 'wss://');

  // ========== WebSocket ==========

  Future<void> connectWs() async {
    _ws = WebSocketChannel.connect(Uri.parse('$_wsUrl/ws'));
    _connected = true;
    _ws!.stream.listen(
      (data) {
        if (data is String) {
          try {
            _messageController.add(jsonDecode(data) as Map<String, dynamic>);
          } catch (_) {}
        } else if (data is List<int>) {
          _handleBinaryFrame(Uint8List.fromList(data));
        }
      },
      onDone: () {
        _connected = false;
      },
      onError: (_) {
        _connected = false;
      },
    );
  }

  void _handleBinaryFrame(Uint8List data) {
    if (data.isEmpty) return;
    final frameType = data[0];
    if (frameType == 0x02) {
      // Voice frame: [0x02][JSON meta][0x00][audio bytes]
      final nullIdx = data.indexOf(0, 1);
      if (nullIdx > 1) {
        final metaJson = jsonDecode(String.fromCharCodes(data.sublist(1, nullIdx)));
        _messageController.add({
          'type': 'voice',
          'meta': metaJson,
          'audioLength': data.length - nullIdx - 1,
        });
      }
    } else if (frameType == 0x03) {
      // Stream frame: [0x03][4B ID len][streamID][4B frame#][payload]
      if (data.length > 9) {
        final idLen = (data[1] << 24) | (data[2] << 16) | (data[3] << 8) | data[4];
        final streamId = String.fromCharCodes(data.sublist(5, 5 + idLen));
        final frameNum = (data[5 + idLen] << 24) |
            (data[6 + idLen] << 16) |
            (data[7 + idLen] << 8) |
            data[8 + idLen];
        _messageController.add({
          'type': 'stream-frame',
          'streamId': streamId,
          'frameNum': frameNum,
          'payloadLength': data.length - 9 - idLen,
        });
      }
    }
  }

  void disconnect() {
    _ws?.sink.close();
    _connected = false;
  }

  void _sendWs(Map<String, dynamic> msg) {
    if (_connected && _ws != null) {
      _ws!.sink.add(jsonEncode(msg));
    }
  }

  void sendText(String text, {String? to, String? channel}) {
    _sendWs({
      'type': 'chat',
      'text': text,
      if (to != null) 'to': to,
      if (channel != null) 'channel': channel,
      'ts': DateTime.now().millisecondsSinceEpoch ~/ 1000,
    });
  }

  void sendVoice(Uint8List audioBytes, int durationMs, {String? to}) {
    if (!_connected || _ws == null) return;
    final meta = jsonEncode({
      'type': 'voice',
      'duration': durationMs,
      if (to != null) 'to': to,
    });
    final metaBytes = Uint8List.fromList(utf8.encode(meta));
    final frame = Uint8List(1 + metaBytes.length + 1 + audioBytes.length);
    frame[0] = 0x02;
    frame.setRange(1, 1 + metaBytes.length, metaBytes);
    frame[1 + metaBytes.length] = 0x00;
    frame.setRange(2 + metaBytes.length, frame.length, audioBytes);
    _ws!.sink.add(frame);
  }

  // ========== REST API ==========

  Future<Map<String, dynamic>> getStatus() async {
    final r = await http.get(Uri.parse('$baseUrl/api/status'));
    return jsonDecode(r.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> createIdentity() async {
    final r = await http.post(Uri.parse('$baseUrl/api/identity'));
    return jsonDecode(r.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getIdentity() async {
    final r = await http.get(Uri.parse('$baseUrl/api/identity'));
    return jsonDecode(r.body) as Map<String, dynamic>;
  }

  Future<List<dynamic>> getPeers() async {
    final r = await http.get(Uri.parse('$baseUrl/api/peers'));
    return jsonDecode(r.body) as List<dynamic>;
  }

  Future<List<dynamic>> getChannels() async {
    final r = await http.get(Uri.parse('$baseUrl/api/channels'));
    return jsonDecode(r.body) as List<dynamic>;
  }

  Future<List<dynamic>> getMessages({String? channel, int limit = 50, int offset = 0}) async {
    final qs = 'limit=$limit&offset=$offset${channel != null ? '&channel=$channel' : ''}';
    final r = await http.get(Uri.parse('$baseUrl/api/messages?$qs'));
    return jsonDecode(r.body) as List<dynamic>;
  }

  Future<Map<String, dynamic>> sendMessage({
    required String text,
    String? to,
    String? channel,
    int? ttl,
  }) async {
    final body = <String, dynamic>{
      'from': 'sdk',
      'text': text,
      if (to != null) 'to': to,
      if (channel != null) 'channel': channel,
      if (ttl != null) 'ttl': ttl,
    };
    final r = await http.post(
      Uri.parse('$baseUrl/api/messages'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode(body),
    );
    return jsonDecode(r.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> uploadFile(String filePath, String filename) async {
    final request = http.MultipartRequest('POST', Uri.parse('$baseUrl/api/files/upload'))
      ..fields['from'] = 'sdk'
      ..files.add(await http.MultipartFile.fromPath('file', filePath));
    final r = await request.send();
    final body = await r.stream.bytesToString();
    return jsonDecode(body) as Map<String, dynamic>;
  }

  Future<List<dynamic>> getStreamList() async {
    final r = await http.get(Uri.parse('$baseUrl/api/stream/list'));
    return jsonDecode(r.body) as List<dynamic>;
  }

  Future<List<dynamic>> getStickerPacks() async {
    final r = await http.get(Uri.parse('$baseUrl/api/stickers/packs'));
    return jsonDecode(r.body) as List<dynamic>;
  }

  Future<List<dynamic>> getBotList() async {
    final r = await http.get(Uri.parse('$baseUrl/api/bots/list'));
    return jsonDecode(r.body) as List<dynamic>;
  }

  Future<Map<String, dynamic>> getMeshStats() async {
    final r = await http.get(Uri.parse('$baseUrl/api/mesh/stats'));
    return jsonDecode(r.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getNostrStats() async {
    final r = await http.get(Uri.parse('$baseUrl/api/nostr/stats'));
    return jsonDecode(r.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getNatDiscover() async {
    final r = await http.get(Uri.parse('$baseUrl/api/nat/discover'));
    return jsonDecode(r.body) as Map<String, dynamic>;
  }
}
