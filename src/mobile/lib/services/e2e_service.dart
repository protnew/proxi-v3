import 'dart:convert';
import 'dart:typed_data';
import 'package:pointycastle/pointycastle.dart' as pc;

class E2EService {
  // ── X25519 key pair generation ────────────────────────────────────

  /// Generates a Curve25519 key pair.
  /// Returns { 'private_key': base64, 'public_key': base64 }
  Map<String, String> generateKeyPair() {
    final secureRandom = _getSecureRandom();
    final keyParams = pc.ECDomainParameters('curve25519');

    final keyGen = pc.KeyGenerator('X25519')
      ..init(pc.ParametersWithRandom(
        pc.ECKeyGeneratorParameters(keyParams),
        secureRandom,
      ));

    final pair = keyGen.generateKeyPair();
    final privateKey = pair.privateKey as pc_ECPrivateKey;
    final publicKey = pair.publicKey as pc_ECPublicKey;

    return {
      'private_key': base64Encode(privateKey.d!),
      'public_key': base64Encode(publicKey.q!.getEncoded(false)),
    };
  }

  /// Derive a shared secret using X25519 Diffie-Hellman
  Uint8List deriveSharedSecret(String myPrivateKeyB64, String peerPublicKeyB64) {
    final myPrivate = base64Decode(myPrivateKeyB64);
    final peerPublic = base64Decode(peerPublicKeyB64);

    final domain = pc.ECDomainParameters('curve25519');
    final privateKey = pc_ECPrivateKey(BigInt.from(0), domain)
      ..d = myPrivate;

    final publicKey = pc_ECPublicKey(
      domain.curve.decodePoint(peerPublic),
      domain,
    );

    final ecAgreement = pc.KeyAgreement('X25519')
      ..init(pc.PrivateKeyParameter<pc_ECPrivateKey>(privateKey));

    final shared = ecAgreement.calculateSharedSecret(
      pc.PublicKeyParameter<pc_ECPublicKey>(publicKey),
    );

    return shared;
  }

  // ── AES-256-GCM encrypt / decrypt ────────────────────────────────

  /// Encrypt plaintext using AES-256-GCM with a derived key.
  /// Returns base64( iv + ciphertext + tag ).
  String encrypt(String plaintext, Uint8List key) {
    final iv = _randomBytes(12);
    final cipher = pc.PaddedBlockCipher('AES/GCM')
      ..init(
        true,
        pc.PaddedBlockCipherParameters(
          pc.ParametersWithIV(pc.KeyParameter(key), iv),
          null,
        ),
      );

    final input = Uint8List.fromList(utf8.encode(plaintext));
    final encrypted = cipher.process(input);

    final output = Uint8List(iv.length + encrypted.length);
    output.setRange(0, iv.length, iv);
    output.setRange(iv.length, output.length, encrypted);

    return base64Encode(output);
  }

  String decrypt(String encryptedB64, Uint8List key) {
    final raw = base64Decode(encryptedB64);
    final iv = raw.sublist(0, 12);
    final cipherText = raw.sublist(12);

    final cipher = pc.PaddedBlockCipher('AES/GCM')
      ..init(
        false,
        pc.PaddedBlockCipherParameters(
          pc.ParametersWithIV(pc.KeyParameter(key), iv),
          null,
        ),
      );

    final decrypted = cipher.process(Uint8List.fromList(cipherText));
    return utf8Decode(decrypted);
  }

  /// HKDF-SHA256 key derivation
  Uint8List deriveKey(Uint8List sharedSecret, Uint8List salt, {int length = 32}) {
    final hmac = pc.HMac(pc.SHA256Digest(), 64);
    hmac.init(pc.KeyParameter(salt));
    hmac.update(sharedSecret, 0, sharedSecret.length);

    final output = Uint8List(length);
    hmac.doFinal(output, 0);
    return output;
  }

  // ── Helpers ───────────────────────────────────────────────────────

  Uint8List _randomBytes(int length) {
    final secureRandom = _getSecureRandom();
    final bytes = Uint8List(length);
    for (var i = 0; i < length; i++) {
      bytes[i] = secureRandom.nextUint8();
    }
    return bytes;
  }

  pc.SecureRandom _getSecureRandom() {
    final secureRandom = pc.SecureRandom('Fortuna')
      ..seed(pc.KeyParameter(Uint8List.fromList(List.generate(32, (_) => 0))));
    return secureRandom;
  }

  String utf8Decode(Uint8List data) {
    return utf8.decode(data, allowMalformed: true);
  }
}

// Re-export PointyCastle private key / public key types
// ignore: non_constant_identifier_names
typedef pc_ECPrivateKey = pc.ECPrivateKey;
// ignore: non_constant_identifier_names
typedef pc_ECPublicKey = pc.ECPublicKey;
