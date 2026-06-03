import 'message.dart';
import 'user.dart';

class Chat {
  final String id;
  final String name;
  final User? peer;
  final Message? lastMessage;
  final int unreadCount;
  final bool isGroup;
  final bool isE2E;
  final String? avatarUrl;
  final DateTime updatedAt;

  Chat({
    required this.id,
    required this.name,
    this.peer,
    this.lastMessage,
    this.unreadCount = 0,
    this.isGroup = false,
    this.isE2E = false,
    this.avatarUrl,
    DateTime? updatedAt,
  }) : updatedAt = updatedAt ?? DateTime.now();

  Chat copyWith({
    String? name,
    Message? lastMessage,
    int? unreadCount,
    bool? isE2E,
    String? avatarUrl,
    DateTime? updatedAt,
  }) {
    return Chat(
      id: id,
      name: name ?? this.name,
      peer: peer,
      lastMessage: lastMessage ?? this.lastMessage,
      unreadCount: unreadCount ?? this.unreadCount,
      isGroup: isGroup,
      isE2E: isE2E ?? this.isE2E,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  factory Chat.fromJson(Map<String, dynamic> json) {
    return Chat(
      id: json['id'] as String,
      name: json['name'] as String? ?? '',
      peer: json['peer'] != null ? User.fromJson(json['peer'] as Map<String, dynamic>) : null,
      lastMessage: json['last_message'] != null
          ? Message.fromJson(json['last_message'] as Map<String, dynamic>)
          : null,
      unreadCount: json['unread_count'] as int? ?? 0,
      isGroup: json['is_group'] as bool? ?? false,
      isE2E: json['is_e2e'] as bool? ?? false,
      avatarUrl: json['avatar_url'] as String?,
      updatedAt: json['updated_at'] != null
          ? DateTime.parse(json['updated_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'peer': peer?.toJson(),
        'last_message': lastMessage?.toJson(),
        'unread_count': unreadCount,
        'is_group': isGroup,
        'is_e2e': isE2E,
        'avatar_url': avatarUrl,
        'updated_at': updatedAt.toIso8601String(),
      };
}
