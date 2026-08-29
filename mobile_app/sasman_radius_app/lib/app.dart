import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:provider/provider.dart';

import 'config/app_theme.dart';
import 'l10n/app_localizations.dart';
import 'providers/language_provider.dart';
import 'providers/theme_provider.dart';
import 'screens/splash_screen.dart';

class AppMaterialLocalizationsDelegate extends LocalizationsDelegate<MaterialLocalizations> {
  const AppMaterialLocalizationsDelegate();

  static const _supported = <String>{
    'en', 'ar', 'tr', 'fa', 'fr', 'de', 'es', 'ru'
  };

  @override
  bool isSupported(Locale locale) => true;

  @override
  Future<MaterialLocalizations> load(Locale locale) async {
    final target = _supported.contains(locale.languageCode)
        ? locale
        : const Locale('en');
    final delegate = GlobalMaterialLocalizations.delegate;
    return await delegate.load(target);
  }

  @override
  bool shouldReload(covariant LocalizationsDelegate<MaterialLocalizations> old) => false;
}

class SasmanRadiusApp extends StatelessWidget {
  const SasmanRadiusApp({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<LanguageProvider, ThemeProvider>(
      builder: (context, langProvider, themeProvider, _) {
        return MaterialApp(
          title: 'SASMAN RADIUS',
          debugShowCheckedModeBanner: false,
          locale: langProvider.locale,
          supportedLocales: const [
            Locale('ar'),
            Locale('en'),
            Locale('tr'),
            Locale('ku'),
            Locale('fa'),
            Locale('fr'),
            Locale('de'),
            Locale('es'),
            Locale('ru'),
          ],
          localizationsDelegates: const [
            AppLocalizations.delegate,
            AppMaterialLocalizationsDelegate(),
            GlobalWidgetsLocalizations.delegate,
            GlobalCupertinoLocalizations.delegate,
          ],
          theme: AppTheme.light(),
          darkTheme: AppTheme.dark(),
          themeMode: themeProvider?.themeMode ?? ThemeMode.dark,
          home: Directionality(
            textDirection:
                langProvider.isRtl ? TextDirection.rtl : TextDirection.ltr,
            child: const SplashScreen(),
          ),
        );
      },
    );
  }
}