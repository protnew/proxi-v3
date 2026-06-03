import 'package:flutter/material.dart';
import '../models/channel.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';
import '../services/theme.dart';
import '../widgets/contact_avatar.dart';

class ChannelsScreen extends StatefulWidget {
  const ChannelsScreen({super.key});

  @override
  State<ChannelsScreen> createState() => _ChannelsScreenState();
}

class _ChannelsScreenState extends State<ChannelsScreen> {
  final _apiService = ApiService();
  final _storageService = StorageService();
  List<Channel> _channels = [];
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadChannels();
  }

  Future<void> _loadChannels() async {
    try {
      await _storageService.init();
      final token = _storageService.getAuthToken();
      if (token != null) _apiService.setAuthToken(token);

      final raw = await _apiService.getChannels();
      setState(() {
        _channels = raw.map((j) => Channel.fromJson(j as Map<String, dynamic>)).toList();
        _isLoading = false;
      });
    } catch (e) {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _createChannel() async {
    final result = await showDialog<Map<String, String>>(
      context: context,
      builder: (ctx) {
        final nameCtrl = TextEditingController();
        final descCtrl = TextEditingController();
        return AlertDialog(
          title: const Text('Create Channel'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: nameCtrl,
                style: const TextStyle(color: ProxiTheme.onBackground),
                decoration: const InputDecoration(
                  hintText: 'Channel name',
                  prefixIcon: Icon(Icons.campaign, color: ProxiTheme.muted),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: descCtrl,
                style: const TextStyle(color: ProxiTheme.onBackground),
                decoration: const InputDecoration(
                  hintText: 'Description (optional)',
                ),
                maxLines: 3,
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Cancel'),
            ),
            ElevatedButton(
              onPressed: () => Navigator.pop(ctx, {
                'name': nameCtrl.text.trim(),
                'description': descCtrl.text.trim(),
              }),
              child: const Text('Create'),
            ),
          ],
        );
      },
    );

    if (result != null && result['name']!.isNotEmpty) {
      try {
        await _apiService.createChannel(
          result['name']!,
          description: result['description']!.isEmpty ? null : result['description'],
        );
        await _loadChannels();
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Failed: $e')),
          );
        }
      }
    }
  }

  Future<void> _toggleSubscription(Channel channel) async {
    try {
      if (channel.isSubscribed) {
        await _apiService.unsubscribe(channel.id);
      } else {
        await _apiService.subscribe(channel.id);
      }
      await _loadChannels();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed: $e')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator(color: ProxiTheme.primary));
    }

    return Column(
      children: [
        // Header
        Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Channels',
                style: Theme.of(context).textTheme.titleLarge,
              ),
              ElevatedButton.icon(
                onPressed: _createChannel,
                icon: const Icon(Icons.add, size: 18),
                label: const Text('Create'),
                style: ElevatedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                ),
              ),
            ],
          ),
        ),

        // Channel list
        Expanded(
          child: _channels.isEmpty
              ? Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(Icons.campaign_outlined, size: 64, color: ProxiTheme.muted.withValues(alpha: 0.5)),
                      const SizedBox(height: 16),
                      Text(
                        'No channels yet',
                        style: Theme.of(context).textTheme.titleMedium?.copyWith(color: ProxiTheme.muted),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Create or subscribe to a channel',
                        style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: ProxiTheme.muted),
                      ),
                    ],
                  ),
                )
              : RefreshIndicator(
                  color: ProxiTheme.primary,
                  onRefresh: _loadChannels,
                  child: ListView.builder(
                    padding: const EdgeInsets.only(top: 8),
                    itemCount: _channels.length,
                    itemBuilder: (context, index) {
                      final channel = _channels[index];
                      return _ChannelTile(
                        channel: channel,
                        onToggle: () => _toggleSubscription(channel),
                      );
                    },
                  ),
                ),
        ),
      ],
    );
  }
}

class _ChannelTile extends StatelessWidget {
  final Channel channel;
  final VoidCallback onToggle;

  const _ChannelTile({required this.channel, required this.onToggle});

  @override
  Widget build(BuildContext context) {
    return ListTile(
      onTap: () {
        // TODO: Open channel view
      },
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      leading: ContactAvatar(
        avatarUrl: channel.avatarUrl,
        name: channel.name,
        radius: 26,
      ),
      title: Row(
        children: [
          Expanded(
            child: Text(
              channel.name,
              style: const TextStyle(
                color: ProxiTheme.onBackground,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
      subtitle: Row(
        children: [
          const Icon(Icons.people, size: 14, color: ProxiTheme.muted),
          const SizedBox(width: 4),
          Text(
            '${channel.membersCount} subscribers',
            style: const TextStyle(color: ProxiTheme.muted, fontSize: 13),
          ),
          if (channel.description != null && channel.description!.isNotEmpty) ...[
            const SizedBox(width: 8),
            Text(
              '• ${channel.description!}',
              style: const TextStyle(color: ProxiTheme.muted, fontSize: 13),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ],
      ),
      trailing: GestureDetector(
        onTap: onToggle,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
          decoration: BoxDecoration(
            color: channel.isSubscribed ? ProxiTheme.primary.withValues(alpha: 0.15) : ProxiTheme.primary,
            borderRadius: BorderRadius.circular(16),
          ),
          child: Text(
            channel.isSubscribed ? 'Joined' : 'Join',
            style: TextStyle(
              color: channel.isSubscribed ? ProxiTheme.primary : Colors.white,
              fontWeight: FontWeight.w600,
              fontSize: 13,
            ),
          ),
        ),
      ),
    );
  }
}
