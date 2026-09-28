import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import '../services/theme.dart';

class ContactAvatar extends StatelessWidget {
  final String? avatarUrl;
  final String name;
  final double radius;
  final bool isOnline;

  const ContactAvatar({
    super.key,
    this.avatarUrl,
    required this.name,
    this.radius = 24,
    this.isOnline = false,
  });

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        CircleAvatar(
          radius: radius,
          backgroundColor: _colorFromName(name),
          backgroundImage: avatarUrl != null
              ? CachedNetworkImageProvider(avatarUrl!)
              : null,
          child: avatarUrl == null
              ? Text(
                  _initials(name),
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: radius * 0.8,
                    fontWeight: FontWeight.w600,
                  ),
                )
              : null,
        ),
        if (isOnline)
          Positioned(
            right: 0,
            bottom: 0,
            child: Container(
              width: radius * 0.55,
              height: radius * 0.55,
              decoration: BoxDecoration(
                color: ProxiTheme.secondary,
                shape: BoxShape.circle,
                border: Border.all(
                  color: ProxiTheme.background,
                  width: 2,
                ),
              ),
            ),
          ),
      ],
    );
  }

  String _initials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.length >= 2) {
      return '${parts[0][0]}${parts[1][0]}'.toUpperCase();
    }
    return name.isNotEmpty ? name[0].toUpperCase() : '?';
  }

  Color _colorFromName(String name) {
    final hash = name.hashCode;
    final colors = [
      const Color(0xFFE05555),
      const Color(0xFFE8A640),
      const Color(0xFF64DCA0),
      const Color(0xFF6AB2F3),
      const Color(0xFFB06CD0),
      const Color(0xFFE87090),
    ];
    return colors[hash.abs() % colors.length];
  }
}
