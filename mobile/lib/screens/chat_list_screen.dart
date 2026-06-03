import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/chat.dart';
import '../models/message.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';
import '../services/theme.dart';
import '../widgets/contact_avatar.dart';
import 'chat_screen.dart';
import 'channels_screen.dart';
import 'settings_screen.dart';

class ChatListScreen extends StatefulWidget {
  const ChatListScreen({super.key});

  @override
  State<ChatListScreen> createState() => _ChatListScreenState();
}

class _ChatListScreenState extends State<ChatListScreen> {
  final _apiService = ApiService();
  final _storageService = StorageService();
  List<Chat> _chats = [];
  bool _isLoading = true;
  int _currentIndex = 0;

  @override
  void initState() {
    super.initState();
    _loadChats();
  }

  Future<void> _loadChats() async {
    try {
      await _storageService.init();
      // Try API first, fallback to cache
      try {
        final token = _storageService.getAuthToken();
        if (token != null) _apiService.setAuthToken(token);

        final raw = await _apiService.getChats();
        setState(() {
          _chats = raw.map((j) => Chat.fromJson(j as Map<String, dynamic>)).toList();
          _isLoading = false;
        });

        await _storageService.cacheChats(raw.cast<Map<String, dynamic>>());
      } catch (_) {
        // Offline: load from cache
        final cached = _storageService.getCachedChats();
        setState(() {
          _chats = cached.map((j) => Chat.fromJson(j)).toList();
          _isLoading = false;
        });
      }
    } catch (e) {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _createNewChat() async {
    final peerId = await showDialog<String>(
      context: context,
      builder: (ctx) {
        final controller = TextEditingController();
        return AlertDialog(
          title: const Text('New Chat'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: controller,
                style: const TextStyle(color: ProxiTheme.onBackground),
                decoration: const InputDecoration(
                  hintText: 'Enter user ID or username',
                  prefixIcon: Icon(Icons.person_add, color: ProxiTheme.muted),
                ),
              ),
              const SizedBox(height: 8),
              Row(
                children: [
                  Icon(Icons.lock_outline, size: 16, color: ProxiTheme.secondary),
                  const SizedBox(width: 4),
                  Text(
                    'E2E encrypted by default',
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          color: ProxiTheme.secondary,
                        ),
                  ),
                ],
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Cancel'),
            ),
            ElevatedButton(
              onPressed: () => Navigator.pop(ctx, controller.text.trim()),
              child: const Text('Create'),
            ),
          ],
        );
      },
    );

    if (peerId != null && peerId.isNotEmpty) {
      try {
        final response = await _apiService.createChat(peerId, e2e: true);
        await _loadChats();
        if (mounted) {
          final chat = Chat.fromJson(response);
          Navigator.push(
            context,
            MaterialPageRoute(builder: (_) => ChatScreen(chat: chat)),
          );
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Failed to create chat: $e')),
          );
        }
      }
    }
  }

  void _openChat(Chat chat) {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => ChatScreen(chat: chat)),
    ).then((_) => _loadChats());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Proxi Messenger'),
        actions: [
          IconButton(
            icon: const Icon(Icons.search),
            onPressed: () {
              showSearch(
                context: context,
                delegate: _ChatSearchDelegate(_chats, _openChat),
              );
            },
          ),
        ],
      ),
      body: _buildBody(),
      floatingActionButton: _currentIndex == 0
          ? FloatingActionButton(
              onPressed: _createNewChat,
              child: const Icon(Icons.edit),
            )
          : null,
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentIndex,
        onTap: (i) => setState(() => _currentIndex = i),
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.chat_bubble_outline),
            activeIcon: Icon(Icons.chat_bubble),
            label: 'Chats',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.channel_outlined),
            activeIcon: Icon(Icons.channel),
            label: 'Channels',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.settings_outlined),
            activeIcon: Icon(Icons.settings),
            label: 'Settings',
          ),
        ],
      ),
    );
  }

  Widget _buildBody() {
    switch (_currentIndex) {
      case 0:
        return _buildChatList();
      case 1:
        return const ChannelsScreen();
      case 2:
        return const SettingsScreen();
      default:
        return _buildChatList();
    }
  }

  Widget _buildChatList() {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator(color: ProxiTheme.primary));
    }

    if (_chats.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.chat_bubble_outline, size: 64, color: ProxiTheme.muted.withValues(alpha: 0.5)),
            const SizedBox(height: 16),
            Text(
              'No chats yet',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(color: ProxiTheme.muted),
            ),
            const SizedBox(height: 8),
            Text(
              'Tap + to start a new conversation',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: ProxiTheme.muted),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      color: ProxiTheme.primary,
      onRefresh: _loadChats,
      child: ListView.builder(
        padding: const EdgeInsets.only(top: 8),
        itemCount: _chats.length,
        itemBuilder: (context, index) {
          final chat = _chats[index];
          return _ChatTile(
            chat: chat,
            onTap: () => _openChat(chat),
          );
        },
      ),
    );
  }
}

class _ChatTile extends StatelessWidget {
  final Chat chat;
  final VoidCallback onTap;

  const _ChatTile({required this.chat, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return ListTile(
      onTap: onTap,
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      leading: ContactAvatar(
        avatarUrl: chat.avatarUrl,
        name: chat.name,
        radius: 28,
        isOnline: chat.peer?.isOnline ?? false,
      ),
      title: Row(
        children: [
          Expanded(
            child: Text(
              chat.name,
              style: const TextStyle(
                color: ProxiTheme.onBackground,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          Text(
            _formatTime(chat.updatedAt),
            style: TextStyle(
              color: chat.unreadCount > 0 ? ProxiTheme.primary : ProxiTheme.muted,
              fontSize: 12,
            ),
          ),
        ],
      ),
      subtitle: Row(
        children: [
          if (chat.isE2E)
            const Padding(
              padding: EdgeInsets.only(right: 4),
              child: Icon(Icons.lock, size: 12, color: ProxiTheme.secondary),
            ),
          Expanded(
            child: Text(
              chat.lastMessage?.text ?? 'No messages yet',
              style: TextStyle(
                color: chat.unreadCount > 0 ? ProxiTheme.onSurface : ProxiTheme.muted,
                fontSize: 14,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          if (chat.unreadCount > 0)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
              decoration: BoxDecoration(
                color: ProxiTheme.primary,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Text(
                '${chat.unreadCount}',
                style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w600),
              ),
            ),
        ],
      ),
    );
  }

  String _formatTime(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inDays == 0) {
      return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    } else if (diff.inDays == 1) {
      return 'Yesterday';
    } else if (diff.inDays < 7) {
      const days = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
      return days[dt.weekday - 1];
    } else {
      return '${dt.day.toString().padLeft(2, '0')}.${dt.month.toString().padLeft(2, '0')}';
    }
  }
}

class _ChatSearchDelegate extends SearchDelegate {
  final List<Chat> chats;
  final void Function(Chat) onOpen;

  _ChatSearchDelegate(this.chats, this.onOpen);

  @override
  ThemeData appBarTheme(BuildContext context) {
    return Theme.of(context).copyWith(
      appBarTheme: const AppBarTheme(backgroundColor: ProxiTheme.surface),
      inputDecorationTheme: const InputDecorationTheme(
        hintStyle: TextStyle(color: ProxiTheme.muted),
      ),
    );
  }

  @override
  List<Widget> buildActions(BuildContext context) => [
        IconButton(icon: const Icon(Icons.clear), onPressed: () => query = ''),
      ];

  @override
  Widget buildLeading(BuildContext context) =>
      IconButton(icon: const Icon(Icons.arrow_back), onPressed: () => close(context, null));

  @override
  Widget buildResults(BuildContext context) => _buildList();

  @override
  Widget buildSuggestions(BuildContext context) => _buildList();

  Widget _buildList() {
    final filtered = query.isEmpty
        ? chats
        : chats
            .where((c) => c.name.toLowerCase().contains(query.toLowerCase()))
            .toList();
    return ListView.builder(
      itemCount: filtered.length,
      itemBuilder: (context, i) => _ChatTile(
        chat: filtered[i],
        onTap: () {
          close(context, null);
          onOpen(filtered[i]);
        },
      ),
    );
  }
}
