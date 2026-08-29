import 'package:flutter/material.dart';

/// Manages the app's current locale and persists the selection.
class LanguageProvider extends ChangeNotifier {
  Locale _locale = const Locale('ar'); // Default Arabic

  Locale get locale => _locale;

  /// Returns true if the current locale is RTL (Arabic, Kurdish, Persian).
  bool get isRtl =>
      const ['ar', 'ku', 'fa'].contains(_locale.languageCode);

  /// Ordered list of all supported locales, used for cycling languages.
  static const List<Locale> supportedLocales = [
    Locale('ar'),
    Locale('en'),
    Locale('tr'),
    Locale('ku'),
    Locale('fa'),
    Locale('fr'),
    Locale('de'),
    Locale('es'),
    Locale('ru'),
  ];

  LanguageProvider() {
    // Could restore from SharedPreferences in production.
  }

  void setLocale(Locale locale) {
    if (_locale == locale) return;
    _locale = locale;
    notifyListeners();
  }

  /// Cycle through available languages in a fixed, predictable order.
  void cycleLocale() {
    final index =
        supportedLocales.indexWhere((l) => l.languageCode == _locale.languageCode);
    final next = supportedLocales[(index + 1) % supportedLocales.length];
    setLocale(next);
  }

  /// Returns a display name for the current language (shown on the button).
  String get currentLanguageName {
    switch (_locale.languageCode) {
      case 'en':
        return 'EN';
      case 'tr':
        return 'TR';
      case 'ku':
        return 'KU';
      case 'fa':
        return 'FA';
      case 'fr':
        return 'FR';
      case 'de':
        return 'DE';
      case 'es':
        return 'ES';
      case 'ru':
        return 'RU';
      case 'ar':
      default:
        return 'عربي';
    }
  }

  /// Returns the flag emoji for the current language.
  String get currentFlag {
    switch (_locale.languageCode) {
      case 'en':
        return '🇺🇸';
      case 'tr':
        return '🇹🇷';
      case 'ku':
        return '🇹🇯';
      case 'fa':
        return '🇮🇷';
      case 'fr':
        return '🇫🇷';
      case 'de':
        return '🇩🇪';
      case 'es':
        return '🇪🇸';
      case 'ru':
        return '🇷🇺';
      case 'ar':
      default:
        return '🇮🇶';
    }
  }
}
