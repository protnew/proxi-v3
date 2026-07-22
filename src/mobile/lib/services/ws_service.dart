import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';

typedef WsMessageHandler = void Function(Map<String, dynamic> data);

class WsService {
  static const String _wsUrl = 'ws://localhost:9999/ws';

  WebSocketChannel? _channel;
  String? _authToken;
  Timer? _reconnectTimer;
  Timer? _heartbeatTimer;
  bool _disposed = false;

  final List<WsMessageHandler> _handlers = [];
  final StreamController<ConnectionState> _connectionStateController =
      StreamController<ConnectionState>.broadcast();

  Stream<ConnectionState> get connectionState => _connectionStateController.stream;

  void setAuthToken(String token) {
    _authToken = token;
  }

  void connect() {
    if (_disposed) return;
    _cleanup();

    final uri = Uri.parse('$_wsUrl?token=$_authToken');
    _connectionStateController.add(ConnectionState.connecting);

    try {
      _channel = WebSocketChannel.connect(uri);

      _channel!.stream.listen(
        (data) {
          try {
            final json = jsonDecode(data as String) as Map<String, dynamic>;
            _dispatch(json);
          } catch (_) {
            // ignore malformed
          }
        },
        onError: (Object error) {
          _connectionStateController.add(ConnectionState.disconnected);
          _scheduleReconnect();
        },
        onDone: () {
          _connectionStateController.add(ConnectionState.disconnected);
          _scheduleReconnect();
        },
        cancelOnError: false,
      );

      _connectionStateController.add(ConnectionState.connected);
      _startHeartbeat();
    } catch (e) {
      _connectionStateController.add(ConnectionState.disconnected);
      _scheduleReconnect();
    }
  }

  void addHandler(WsMessageHandler handler) {
    _handlers.add(handler);
  }

  void removeHandler(WsMessageHandler handler) {
    _handlers.remove(handler);
  }

  void send(Map<String, dynamic> data) {
    if (_channel != null) {
      _channel!.sink.add(jsonEncode(data));
    }
  }

  void sendTyping(String chatId) {
    send({'type': 'typing', 'chat_id': chatId});
  }

  void sendMessage({
    required String chatId,
    required String text,
    String? replyTo,
  }) {
    send({
      'type': 'message',
      'chat_id': chatId,
      'text': text,
      if (replyTo != null) 'reply_to': replyTo,
    });
  }

  void _dispatch(Map<String, dynamic> data) {
    for (final handler in _handlers) {
      handler(data);
    }
  }

  void _startHeartbeat() {
    _heartbeatTimer?.cancel();
    _heartbeatTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      send({'type': 'ping'});
    });
  }

  void _scheduleReconnect() {
    if (_disposed) return;
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 3), connect);
  }

  void _cleanup() {
    _heartbeatTimer?.cancel();
    _reconnectTimer?.cancel();
    try {
      _channel?.sink.close();
    } catch (_) {}
    _channel = null;
  }

  void dispose() {
    _disposed = true;
    _cleanup();
    _connectionStateController.close();
  }
}

enum ConnectionState { disconnected, connecting, connected }
