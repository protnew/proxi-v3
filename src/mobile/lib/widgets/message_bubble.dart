import 'package:flutter/material.dart';
import '../models/message.dart';
import '../services/theme.dart';

class MessageBubble extends StatelessWidget {
  final Message message;
  final VoidCallback? onTap;
  final VoidCallback? onLongPress;
  final VoidCallback? onReplyTap;
  final bool showAvatar;
  final String? senderName;

  const MessageBubble({
    super.key,
    required this.message,
    this.onTap,
    this.onLongPress,
    this.onReplyTap,
    this.showAvatar = true,
    this.senderName,
  });

  @override
  Widget build(BuildContext context) {
    final isOwn = message.isOwn;
    final theme = Theme.of(context);

    return Align(
      alignment: isOwn ? Alignment.centerRight : Alignment.centerLeft,
      child: GestureDetector(
        onTap: onTap,
        onLongPress: onLongPress,
        child: Container(
          constraints: BoxConstraints(
            maxWidth: MediaQuery.of(context).size.width * 0.75,
          ),
          margin: EdgeInsets.only(
            left: isOwn ? 48 : 8,
            right: isOwn ? 8 : 48,
            top: 2,
            bottom: 2,
          ),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(
            color: isOwn ? ProxiTheme.bubbleOwn : ProxiTheme.bubblePeer,
            borderRadius: BorderRadius.only(
              topLeft: const Radius.circular(16),
              topRight: const Radius.circular(16),
              bottomLeft: Radius.circular(isOwn ? 16 : 4),
              bottomRight: Radius.circular(isOwn ? 4 : 16),
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              // Reply indicator
              if (message.replyTo != null)
                GestureDetector(
                  onTap: onReplyTap,
                  child: Container(
                    margin: const EdgeInsets.only(bottom: 6),
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: ProxiTheme.primary.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(8),
                      border: Border(
                        left: BorderSide(color: ProxiTheme.primary, width: 3),
                      ),
                    ),
                    child: Text(
                      'Reply to: ${message.replyTo}',
                      style: theme.textTheme.labelSmall?.copyWith(
                        color: ProxiTheme.primary,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ),

              // Sender name for group chats
              if (senderName != null && !isOwn)
                Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Text(
                    senderName!,
                    style: theme.textTheme.labelLarge?.copyWith(fontSize: 12),
                  ),
                ),

              // Attachment
              if (message.type == MessageType.image && message.attachmentUrl != null)
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Container(
                    height: 200,
                    color: ProxiTheme.surfaceVariant,
                    child: const Center(
                      child: Icon(Icons.image, color: ProxiTheme.muted, size: 48),
                    ),
                  ),
                ),

              if (message.type == MessageType.voice)
                _VoiceWidget(duration: message.voiceDuration ?? 0.0),

              if (message.type == MessageType.file)
                _FileWidget(fileName: message.fileName ?? 'File'),

              // Text
              if (message.text.isNotEmpty)
                Text(
                  message.text,
                  style: theme.textTheme.bodyLarge?.copyWith(
                    fontSize: 15,
                    height: 1.4,
                  ),
                ),

              // Timestamp + status
              Row(
                mainAxisSize: MainAxisSize.min,
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  Text(
                    _formatTime(message.createdAt),
                    style: theme.textTheme.labelSmall?.copyWith(fontSize: 11),
                  ),
                  if (isOwn) ...[
                    const SizedBox(width: 4),
                    _statusIcon(message.status),
                  ],
                  if (message.metadata?['encrypted'] == true) ...[
                    const SizedBox(width: 4),
                    const Icon(Icons.lock, size: 12, color: ProxiTheme.secondary),
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _statusIcon(MessageStatus status) {
    switch (status) {
      case MessageStatus.sending:
        return const Icon(Icons.access_time, size: 14, color: ProxiTheme.muted);
      case MessageStatus.sent:
        return const Icon(Icons.check, size: 14, color: ProxiTheme.muted);
      case MessageStatus.delivered:
        return const Icon(Icons.done_all, size: 14, color: ProxiTheme.muted);
      case MessageStatus.read:
        return const Icon(Icons.done_all, size: 14, color: ProxiTheme.primary);
      case MessageStatus.failed:
        return const Icon(Icons.error_outline, size: 14, color: ProxiTheme.danger);
    }
  }

  String _formatTime(DateTime dt) {
    return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
  }
}

class _VoiceWidget extends StatelessWidget {
  final double duration;
  const _VoiceWidget({required this.duration});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.all(8),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.play_circle_fill, color: ProxiTheme.primary, size: 32),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                LinearProgressIndicator(
                  value: 0.3,
                  backgroundColor: ProxiTheme.surfaceVariant,
                  valueColor: const AlwaysStoppedAnimation(ProxiTheme.primary),
                ),
                const SizedBox(height: 4),
                Text(
                  '${(duration ~/ 60)}:${(duration % 60).toInt().toString().padLeft(2, '0')}',
                  style: Theme.of(context).textTheme.labelSmall,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _FileWidget extends StatelessWidget {
  final String fileName;
  const _FileWidget({required this.fileName});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: ProxiTheme.surfaceVariant,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.insert_drive_file, color: ProxiTheme.primary, size: 28),
          const SizedBox(width: 8),
          Flexible(
            child: Text(
              fileName,
              style: const TextStyle(color: ProxiTheme.primary, fontSize: 13),
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }
}
