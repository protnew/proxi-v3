class Identity {
  final String id;
  final String username;
  final String displayName;
  final String? avatarUrl;
  final String privateKeyEncrypted;
  final String publicKey;
  final DateTime createdAt;

  Identity({
    required this.id,
    required this.username,
    this.displayName = '',
    this.avatarUrl,
    required this.privateKeyEncrypted,
    required this.publicKey,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.now();

  Identity copyWith({
    String? displayName,
    String? avatarUrl,
  }) {
    return Identity(
      id: id,
      username: username,
      displayName: displayName ?? this.displayName,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      privateKeyEncrypted: privateKeyEncrypted,
      publicKey: publicKey,
      createdAt: createdAt,
    );
  }

  factory Identity.fromJson(Map<String, dynamic> json) {
    return Identity(
      id: json['id'] as String,
      username: json['username'] as String,
      displayName: json['display_name'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      privateKeyEncrypted: json['private_key_encrypted'] as String,
      publicKey: json['public_key'] as String,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'username': username,
        'display_name': displayName,
        'avatar_url': avatarUrl,
        'private_key_encrypted': privateKeyEncrypted,
        'public_key': publicKey,
        'created_at': createdAt.toIso8601String(),
      };
}
