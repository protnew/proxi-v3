import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'services/theme.dart';
import 'screens/login_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Lock to portrait
  await SystemChrome.setPreferredOrientations([
    DeviceOrientation.portraitUp,
    DeviceOrientation.portraitDown,
  ]);

  // Transparent status bar
  SystemChrome.setSystemUIOverlayStyle(const SystemUiOverlayStyle(
    statusBarColor: Colors.transparent,
    statusBarIconBrightness: Brightness.light,
    statusBarBrightness: Brightness.dark,
  ));

  runApp(const ProxiMessengerApp());
}

class ProxiMessengerApp extends StatelessWidget {
  const ProxiMessengerApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Proxi Messenger',
      debugShowCheckedModeBanner: false,
      theme: ProxiTheme.darkTheme(),
      darkTheme: ProxiTheme.darkTheme(),
      themeMode: ThemeMode.dark,
      home: const LoginScreen(),
      // Named routes
      routes: {
        '/': (_) => const LoginScreen(),
        '/login': (_) => const LoginScreen(),
      },
    );
  }
}
