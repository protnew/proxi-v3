import 'dart:typed_data';
import 'package:cryptography/cryptography.dart';

/// E2E encryption client using X25519 ECDH + AES-256-GCM.
class E2EClient {
  final _x25519 = X25519();
  final _aesGcm = AesGcm.with256bits();

  Future<SimpleKeyPair> generateKeyPair() async {
    return _x25519.newKeyPair();
  }

  Future<SecretKey> deriveSharedSecret(
    SimplePublicKey peerPublic,
    SimpleKeyPair myPrivate,
  ) async {
    return _x25519.sharedSecretKey(
      remotePublicKey: peerPublic,
      localKeyPair: myPrivate,
    );
  }

  Future<Uint8List> encrypt(Uint8List plaintext, SecretKey key) async {
    final secretBox = await _aesGcm.encrypt(plaintext, secretKey: key);
    // Output: nonce(12) + ciphertext + mac(16)
    return Uint8List.fromList([
      ...secretBox.nonce,
      ...secretBox.cipherText,
      ...secretBox.mac.bytes,
    ]);
  }

  Future<Uint8List> decrypt(Uint8List data, SecretKey key) async {
    if (data.length < 28) throw ArgumentError('Ciphertext too short');
    final nonce = data.sublist(0, 12);
    final mac = List<int>.from(data.sublist(data.length - 16));
    final cipherText = data.sublist(12, data.length - 16);
    final secretBox = SecretBox(
      nonce: nonce,
      cipherText: cipherText,
      mac: Mac(mac),
    );
    return Uint8List.fromList(await _aesGcm.decrypt(secretBox, secretKey: key));
  }

  /// Export public key as base64 string.
  Future<String> exportPublicKey(SimpleKeyPair keyPair) async {
    final pub = await keyPair.extractPublicKey();
    return base64Encode(pub.bytes);
  }

  /// Import public key from base64 string.
  SimplePublicKey importPublicKey(String base64) {
    final bytes = base64Decode(base64);
    return SimplePublicKey(bytes, type: KeyPairType.x25519);
  }
}

String base64Encode(List<int> bytes) {
  return Uri.encodeComponent(String.fromCharCodes(bytes));
}

List<int> base64Decode(String encoded) {
  return Uri.decodeComponent(encoded).codeUnits;
}
