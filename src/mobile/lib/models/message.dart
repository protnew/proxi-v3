import 'user.dart';

enum MessageStatus { sending, sent, delivered, read, failed }

enum MessageType { text, image, voice, file, system }

class Message {
  final String id;
  final String chatId;
  final String senderId;
  final String text;
  final MessageType type;
  final MessageStatus status;
  final DateTime createdAt;
  final bool isOwn;
  final String? replyTo;
  final String? attachmentUrl;
  final String? fileName;
  final double? voiceDuration;
  final Map<String, dynamic>? metadata;

  Message({
    required this.id,
    required this.chatId,
    required this.senderId,
    required this.text,
    this.type = MessageType.text,
    this.status = MessageStatus.sent,
    DateTime? createdAt,
    this.isOwn = false,
    this.replyTo,
    this.attachmentUrl,
    this.fileName,
    this.voiceDuration,
    this.metadata,
  }) : createdAt = createdAt ?? DateTime.now();

  Message copyWith({
    String? text,
    MessageStatus? status,
    String? attachmentUrl,
  }) {
    return Message(
      id: id,
      chatId: chatId,
      senderId: senderId,
      text: text ?? this.text,
      type: type,
      status: status ?? this.status,
      createdAt: createdAt,
      isOwn: isOwn,
      replyTo: replyTo,
      attachmentUrl: attachmentUrl ?? this.attachmentUrl,
      fileName: fileName,
      voiceDuration: voiceDuration,
      metadata: metadata,
    );
  }

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] as String,
      chatId: json['chat_id'] as String? ?? '',
      senderId: json['sender_id'] as String,
      text: json['text'] as String? ?? '',
      type: _parseType(json['type'] as String?),
      status: _parseStatus(json['status'] as String?),
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      isOwn: json['is_own'] as bool? ?? false,
      replyTo: json['reply_to'] as String?,
      attachmentUrl: json['attachment_url'] as String?,
      fileName: json['file_name'] as String?,
      voiceDuration: (json['voice_duration'] as num?)?.toDouble(),
      metadata: json['metadata'] as Map<String, dynamic>?,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'chat_id': chatId,
        'sender_id': senderId,
        'text': text,
        'type': type.name,
        'status': status.name,
        'created_at': createdAt.toIso8601String(),
        'is_own': isOwn,
        'reply_to': replyTo,
        'attachment_url': attachmentUrl,
        'file_name': fileName,
        'voice_duration': voiceDuration,
        'metadata': metadata,
      };

  static MessageType _parseType(String? val) {
    switch (val) {
      case 'image':
        return MessageType.image;
      case 'voice':
        return MessageType.voice;
      case 'file':
        return MessageType.file;
      case 'system':
        return MessageType.system;
      default:
        return MessageType.text;
    }
  }

  static MessageStatus _parseStatus(String? val) {
    switch (val) {
      case 'sending':
        return MessageStatus.sending;
      case 'delivered':
        return MessageStatus.delivered;
      case 'read':
        return MessageStatus.read;
      case 'failed':
        return MessageStatus.failed;
      default:
        return MessageStatus.sent;
    }
  }
}
