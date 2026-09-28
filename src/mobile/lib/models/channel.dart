class Channel {
  final String id;
  final String name;
  final String? description;
  final String? avatarUrl;
  final String owner;
  final int membersCount;
  final bool isSubscribed;
  final DateTime createdAt;

  Channel({
    required this.id,
    required this.name,
    this.description,
    this.avatarUrl,
    required this.owner,
    this.membersCount = 0,
    this.isSubscribed = false,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.now();

  Channel copyWith({
    String? name,
    String? description,
    String? avatarUrl,
    int? membersCount,
    bool? isSubscribed,
  }) {
    return Channel(
      id: id,
      name: name ?? this.name,
      description: description ?? this.description,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      owner: owner,
      membersCount: membersCount ?? this.membersCount,
      isSubscribed: isSubscribed ?? this.isSubscribed,
      createdAt: createdAt,
    );
  }

  factory Channel.fromJson(Map<String, dynamic> json) {
    return Channel(
      id: json['id'] as String,
      name: json['name'] as String,
      description: json['description'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      owner: json['owner'] as String? ?? '',
      membersCount: json['members_count'] as int? ?? 0,
      isSubscribed: json['is_subscribed'] as bool? ?? false,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'description': description,
        'avatar_url': avatarUrl,
        'owner': owner,
        'members_count': membersCount,
        'is_subscribed': isSubscribed,
        'created_at': createdAt.toIso8601String(),
      };
}
