class User {
  final String id;
  final String username;
  final String displayName;
  final String? avatarUrl;
  final String? publicKey;
  final bool isOnline;
  final DateTime? lastSeen;
  final String? status;

  User({
    required this.id,
    required this.username,
    this.displayName = '',
    this.avatarUrl,
    this.publicKey,
    this.isOnline = false,
    this.lastSeen,
    this.status,
  });

  String get displayOrUsername => displayName.isNotEmpty ? displayName : username;

  User copyWith({
    String? displayName,
    String? avatarUrl,
    String? publicKey,
    bool? isOnline,
    DateTime? lastSeen,
    String? status,
  }) {
    return User(
      id: id,
      username: username,
      displayName: displayName ?? this.displayName,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      publicKey: publicKey ?? this.publicKey,
      isOnline: isOnline ?? this.isOnline,
      lastSeen: lastSeen ?? this.lastSeen,
      status: status ?? this.status,
    );
  }

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'] as String,
      username: json['username'] as String? ?? '',
      displayName: json['display_name'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      publicKey: json['public_key'] as String?,
      isOnline: json['is_online'] as bool? ?? false,
      lastSeen: json['last_seen'] != null
          ? DateTime.parse(json['last_seen'] as String)
          : null,
      status: json['status'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'username': username,
        'display_name': displayName,
        'avatar_url': avatarUrl,
        'public_key': publicKey,
        'is_online': isOnline,
        'last_seen': lastSeen?.toIso8601String(),
        'status': status,
      };
}
