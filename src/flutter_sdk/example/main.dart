import 'dart:async';
import 'package:proxi_sdk/proxi_sdk.dart';

void main() async {
  final client = ProxiClient(baseUrl: 'http://localhost:9999');

  print('Connecting to Proxi...');
  await client.connectWs();
  print('Connected: ${client.isConnected}');

  // Listen for messages
  final sub = client.messages.listen((msg) {
    print('Received: $msg');
  });

  // Get server status
  try {
    final status = await client.getStatus();
    print('Server status: $status');
  } catch (e) {
    print('Server not available: $e');
  }

  // Send a test message
  client.sendText('Hello from Proxi Flutter SDK!');

  // Wait for responses
  await Future.delayed(Duration(seconds: 5));

  sub.cancel();
  client.disconnect();
  print('Disconnected.');
}
