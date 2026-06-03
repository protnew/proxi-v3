import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import '../models/chat.dart';
import '../models/message.dart';
import '../services/api_client.dart';
import '../services/storage_service.dart';
import '../services/theme.dart';
import '../widgets/message_bubble.dart';
import '../widgets/voice_recorder.dart';
import '../widgets/contact_avatar.dart';

class ChatScreen extends StatefulWidget {
  final Chat chat;

  const ChatScreen({super.key, required this.chat});

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  final _storageService = StorageService();

  List<Message> _messages = [];
  bool _isLoading = true;
  bool _isSending = false;
  bool _e2eEnabled = false;
  String? _replyTo;
  bool _isTyping = false;

  StreamSubscription? _wsSubscription;

  @override
  void initState() {
    super.initState();
    _e2eEnabled = widget.chat.isE2E;
    _init();
  }

  Future<void> _init() async {
    await _storageService.init();
    await _loadMessages();
    _connectWebSocket();
  }

  // ── WebSocket ────────────────────────────────────────────────────

  void _connectWebSocket() {
    final channel = ApiClient.connectWS();
    if (channel == null) return;

    _wsSubscription = channel.stream.listen(
      (data) {
        try {
          final json = jsonDecode(data as String) as Map<String, dynamic>;
          _onWsMessage(json);
        } catch (_) {
          // ignore malformed data
        }
      },
      onError: (_) {
        // Attempt reconnect after a delay
        Future.delayed(const Duration(seconds: 3), () {
          if (mounted) _connectWebSocket();
        });
      },
      onDone: () {
        // Attempt reconnect after a delay
        Future.delayed(const Duration(seconds: 3), () {
          if (mounted) _connectWebSocket();
        });
      },
      cancelOnError: false,
    );
  }

  void _onWsMessage(Map<String, dynamic> data) {
    final type = data['type'] as String?;
    if (type == 'message' && data['chat_id'] == widget.chat.id) {
      final msg = Message.fromJson(data);
      setState(() {
        _messages.add(msg);
      });
      _scrollToBottom();
    } else if (type == 'typing' && data['chat_id'] == widget.chat.id) {
      setState(() => _isTyping = true);
      Future.delayed(const Duration(seconds: 3), () {
        if (mounted) setState(() => _isTyping = false);
      });
    }
  }

  // ── Load messages ────────────────────────────────────────────────

  Future<void> _loadMessages() async {
    try {
      final raw = await ApiClient.getMessages(widget.chat.id);
      setState(() {
        _messages =
            raw.map((j) => Message.fromJson(j as Map<String, dynamic>)).toList();
        _isLoading = false;
      });
      _scrollToBottom();
    } catch (_) {
      // Load from cache
      final cached = _storageService.getCachedMessages(widget.chat.id);
      setState(() {
        _messages = cached.map((j) => Message.fromJson(j)).toList();
        _isLoading = false;
      });
    }
  }

  // ── Send message ─────────────────────────────────────────────────

  Future<void> _sendMessage() async {
    final text = _messageController.text.trim();
    if (text.isEmpty) return;

    setState(() => _isSending = true);
    _messageController.clear();

    // Optimistic insert
    final optimisticMsg = Message(
      id: 'temp_${DateTime.now().millisecondsSinceEpoch}',
      chatId: widget.chat.id,
      senderId: 'me',
      text: text,
      status: MessageStatus.sending,
      isOwn: true,
      replyTo: _replyTo,
      metadata: _e2eEnabled ? {'encrypted': true} : null,
    );

    setState(() {
      _messages.add(optimisticMsg);
      _replyTo = null;
    });
    _scrollToBottom();

    try {
      await ApiClient.sendMessage(widget.chat.id, text);
      setState(() {
        final idx = _messages.indexWhere((m) => m.id == optimisticMsg.id);
        if (idx != -1) {
          _messages[idx] = optimisticMsg.copyWith(status: MessageStatus.sent);
        }
      });
    } catch (_) {
      setState(() {
        final idx = _messages.indexWhere((m) => m.id == optimisticMsg.id);
        if (idx != -1) {
          _messages[idx] = optimisticMsg.copyWith(status: MessageStatus.failed);
        }
      });
    }

    setState(() => _isSending = false);
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  void _onVoiceRecorded(String path, double duration) {
    final msg = Message(
      id: 'temp_${DateTime.now().millisecondsSinceEpoch}',
      chatId: widget.chat.id,
      senderId: 'me',
      text: '🎤 Voice message',
      type: MessageType.voice,
      status: MessageStatus.sending,
      isOwn: true,
      voiceDuration: duration,
      attachmentUrl: path,
    );
    setState(() => _messages.add(msg));
    _scrollToBottom();
  }

  @override
  void dispose() {
    _messageController.dispose();
    _scrollController.dispose();
    _wsSubscription?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.pop(context),
        ),
        title: Row(
          children: [
            ContactAvatar(
              avatarUrl: widget.chat.avatarUrl,
              name: widget.chat.name,
              radius: 18,
              isOnline: widget.chat.peer?.isOnline ?? false,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    widget.chat.name,
                    style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                  ),
                  Text(
                    _isTyping
                        ? 'typing...'
                        : (widget.chat.peer?.isOnline ?? false
                            ? 'online'
                            : 'last seen recently'),
                    style: TextStyle(
                      fontSize: 12,
                      color: _isTyping
                          ? ProxiTheme.primary
                          : ProxiTheme.muted,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
        actions: [
          if (_e2eEnabled)
            const Padding(
              padding: EdgeInsets.only(right: 8),
              child: Icon(Icons.lock, color: ProxiTheme.secondary, size: 20),
            ),
          IconButton(
            icon: const Icon(Icons.more_vert),
            onPressed: () => _showChatOptions(context),
          ),
        ],
      ),
      body: Column(
        children: [
          // Messages list
          Expanded(
            child: _isLoading
                ? const Center(child: CircularProgressIndicator(color: ProxiTheme.primary))
                : Container(
                    decoration: const BoxDecoration(
                      image: DecorationImage(
                        image: AssetImage('assets/icons/chat_bg.png'),
                        fit: BoxFit.cover,
                        opacity: 0.05,
                      ),
                    ),
                    child: ListView.builder(
                      controller: _scrollController,
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      itemCount: _messages.length,
                      itemBuilder: (context, index) {
                        return MessageBubble(
                          message: _messages[index],
                          onLongPress: () => _showMessageOptions(context, _messages[index]),
                        );
                      },
                    ),
                  ),
          ),

          // Reply bar
          if (_replyTo != null)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              color: ProxiTheme.surface,
              child: Row(
                children: [
                  Container(
                    width: 3,
                    height: 32,
                    color: ProxiTheme.primary,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Reply to: $_replyTo',
                      style: const TextStyle(color: ProxiTheme.muted, fontSize: 13),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, size: 18, color: ProxiTheme.muted),
                    onPressed: () => setState(() => _replyTo = null),
                  ),
                ],
              ),
            ),

          // Input bar
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
            decoration: const BoxDecoration(
              color: ProxiTheme.surface,
              border: Border(top: BorderSide(color: ProxiTheme.divider, width: 0.5)),
            ),
            child: SafeArea(
              child: Row(
                children: [
                  // Attachment button
                  IconButton(
                    icon: const Icon(Icons.attach_file, color: ProxiTheme.muted),
                    onPressed: () {
                      // TODO: File picker
                    },
                  ),

                  // Text input
                  Expanded(
                    child: TextField(
                      controller: _messageController,
                      style: const TextStyle(color: ProxiTheme.onBackground, fontSize: 15),
                      minLines: 1,
                      maxLines: 5,
                      decoration: InputDecoration(
                        hintText: 'Message...',
                        filled: true,
                        fillColor: ProxiTheme.surfaceVariant,
                        suffixIcon: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            // E2E toggle
                            GestureDetector(
                              onTap: () => setState(() => _e2eEnabled = !_e2eEnabled),
                              child: Icon(
                                _e2eEnabled ? Icons.lock : Icons.lock_open,
                                size: 18,
                                color: _e2eEnabled ? ProxiTheme.secondary : ProxiTheme.muted,
                              ),
                            ),
                            const SizedBox(width: 8),
                          ],
                        ),
                      ),
                      onSubmitted: (_) => _sendMessage(),
                    ),
                  ),

                  const SizedBox(width: 4),

                  // Send / Voice
                  if (_messageController.text.isNotEmpty || _isSending)
                    IconButton(
                      icon: _isSending
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: ProxiTheme.primary,
                              ),
                            )
                          : const Icon(Icons.send, color: ProxiTheme.primary),
                      onPressed: _isSending ? null : _sendMessage,
                    )
                  else
                    VoiceRecorder(onRecordComplete: _onVoiceRecorded),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showMessageOptions(BuildContext context, Message message) {
    showModalBottomSheet(
      context: context,
      backgroundColor: ProxiTheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.reply, color: ProxiTheme.primary),
              title: const Text('Reply'),
              onTap: () {
                Navigator.pop(ctx);
                setState(() => _replyTo = message.id);
              },
            ),
            ListTile(
              leading: const Icon(Icons.copy, color: ProxiTheme.muted),
              title: const Text('Copy'),
              onTap: () {
                Navigator.pop(ctx);
                // TODO: Copy to clipboard
              },
            ),
            if (message.isOwn)
              ListTile(
                leading: const Icon(Icons.delete, color: ProxiTheme.danger),
                title: const Text('Delete', style: TextStyle(color: ProxiTheme.danger)),
                onTap: () {
                  Navigator.pop(ctx);
                  setState(() => _messages.removeWhere((m) => m.id == message.id));
                },
              ),
          ],
        ),
      ),
    );
  }

  void _showChatOptions(BuildContext context) {
    showModalBottomSheet(
      context: context,
      backgroundColor: ProxiTheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.search, color: ProxiTheme.primary),
              title: const Text('Search messages'),
              onTap: () => Navigator.pop(ctx),
            ),
            ListTile(
              leading: Icon(
                _e2eEnabled ? Icons.lock : Icons.lock_open,
                color: _e2eEnabled ? ProxiTheme.secondary : ProxiTheme.muted,
              ),
              title: Text('E2E Encryption: ${_e2eEnabled ? "ON" : "OFF"}'),
              onTap: () {
                Navigator.pop(ctx);
                setState(() => _e2eEnabled = !_e2eEnabled);
              },
            ),
            ListTile(
              leading: const Icon(Icons.notifications_outlined, color: ProxiTheme.muted),
              title: const Text('Notifications'),
              onTap: () => Navigator.pop(ctx),
            ),
            ListTile(
              leading: const Icon(Icons.delete_outline, color: ProxiTheme.danger),
              title: const Text('Delete chat', style: TextStyle(color: ProxiTheme.danger)),
              onTap: () {
                Navigator.pop(ctx);
                Navigator.pop(context);
              },
            ),
          ],
        ),
      ),
    );
  }
}
