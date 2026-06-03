import 'package:flutter/material.dart';

class ProxiTheme {
  // ── Telegram-style dark color palette ─────────────────────────────
  static const Color _background = Color(0xFF17212B);      // Main BG
  static const Color _surface = Color(0xFF1E2C3A);          // Cards / surfaces
  static const Color _surfaceVariant = Color(0xFF243447);   // Elevated surfaces
  static const Color _primary = Color(0xFF6AB2F3);           // Accent blue
  static const Color _primaryDark = Color(0xFF4A9AE0);
  static const Color _secondary = Color(0xFF64DCA0);         // Online green
  static const Color _onBackground = Color(0xFFE1E5EA);     // Text on BG
  static const Color _onSurface = Color(0xFFB8C4CE);         // Text on surface
  static const Color _muted = Color(0xFF7A8E9C);             // Secondary text
  static const Color _bubbleOwn = Color(0xFF2B5278);         // Own message bubble
  static const Color _bubblePeer = Color(0xFF182533);         // Peer message bubble
  static const Color _danger = Color(0xFFE05555);
  static const Color _warning = Color(0xFFE8A640);
  static const Color _divider = Color(0xFF0E1621);

  static ThemeData darkTheme() {
    final colorScheme = ColorScheme(
      brightness: Brightness.dark,
      primary: _primary,
      onPrimary: Colors.white,
      secondary: _secondary,
      onSecondary: Colors.white,
      error: _danger,
      onError: Colors.white,
      surface: _surface,
      onSurface: _onBackground,
      surfaceContainerHighest: _surfaceVariant,
    );

    return ThemeData(
      useMaterial3: true,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: _background,
      appBarTheme: const AppBarTheme(
        backgroundColor: _surface,
        foregroundColor: _onBackground,
        elevation: 0,
        centerTitle: false,
        titleTextStyle: TextStyle(
          color: _onBackground,
          fontSize: 18,
          fontWeight: FontWeight.w600,
        ),
      ),
      bottomNavigationBarTheme: const BottomNavigationBarThemeData(
        backgroundColor: _surface,
        selectedItemColor: _primary,
        unselectedItemColor: _muted,
        type: BottomNavigationBarType.fixed,
        elevation: 8,
      ),
      cardTheme: CardTheme(
        color: _surface,
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: _surfaceVariant,
        hintStyle: const TextStyle(color: _muted),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: _primary, width: 1.5),
        ),
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: _primary,
          foregroundColor: Colors.white,
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: _primary,
        ),
      ),
      floatingActionButtonTheme: const FloatingActionButtonThemeData(
        backgroundColor: _primary,
        foregroundColor: Colors.white,
      ),
      chipTheme: ChipThemeData(
        backgroundColor: _surfaceVariant,
        labelStyle: const TextStyle(color: _onBackground),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
      dividerTheme: const DividerThemeData(
        color: _divider,
        thickness: 0.5,
      ),
      dialogTheme: DialogTheme(
        backgroundColor: _surface,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: _surfaceVariant,
        contentTextStyle: const TextStyle(color: _onBackground),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
      textTheme: const TextTheme(
        headlineLarge: TextStyle(color: _onBackground, fontSize: 28, fontWeight: FontWeight.w700),
        headlineMedium: TextStyle(color: _onBackground, fontSize: 22, fontWeight: FontWeight.w600),
        titleLarge: TextStyle(color: _onBackground, fontSize: 18, fontWeight: FontWeight.w600),
        titleMedium: TextStyle(color: _onBackground, fontSize: 16, fontWeight: FontWeight.w500),
        bodyLarge: TextStyle(color: _onBackground, fontSize: 15),
        bodyMedium: TextStyle(color: _onSurface, fontSize: 14),
        labelLarge: TextStyle(color: _primary, fontSize: 14, fontWeight: FontWeight.w600),
        labelSmall: TextStyle(color: _muted, fontSize: 12),
      ),
    );
  }

  // ── Custom colors for widgets ─────────────────────────────────────
  static Color get background => _background;
  static Color get surface => _surface;
  static Color get surfaceVariant => _surfaceVariant;
  static Color get primary => _primary;
  static Color get secondary => _secondary;
  static Color get onBackground => _onBackground;
  static Color get onSurface => _onSurface;
  static Color get muted => _muted;
  static Color get bubbleOwn => _bubbleOwn;
  static Color get bubblePeer => _bubblePeer;
  static Color get danger => _danger;
  static Color get warning => _warning;
  static Color get divider => _divider;
}
