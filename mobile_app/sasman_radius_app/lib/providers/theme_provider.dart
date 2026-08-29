import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Manages the app's current theme mode and persists the selection.
class ThemeProvider extends ChangeNotifier {
  static const String _themeKey = 'theme_mode';
  ThemeMode _themeMode = ThemeMode.dark;

  ThemeMode get themeMode => _themeMode;

  bool get isDark => _themeMode == ThemeMode.dark;
  bool get isLight => _themeMode == ThemeMode.light;
  bool get isSystem => _themeMode == ThemeMode.system;

  ThemeProvider() {
    _loadThemeMode();
  }

  /// Load saved theme mode from SharedPreferences
  Future<void> _loadThemeMode() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final themeIndex = prefs.getInt(_themeKey) ?? 1; // Default to dark
      _themeMode = ThemeMode.values[themeIndex.clamp(0, ThemeMode.values.length - 1)];
      notifyListeners();
    } catch (e) {
      // Keep default dark theme on error
    }
  }

  /// Save theme mode to SharedPreferences
  Future<void> _saveThemeMode() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setInt(_themeKey, _themeMode.index);
    } catch (e) {
      // Silently fail if storage not available
    }
  }

  /// Set theme to light mode
  void setLightTheme() {
    _themeMode = ThemeMode.light;
    _saveThemeMode();
    notifyListeners();
  }

  /// Set theme to dark mode
  void setDarkTheme() {
    _themeMode = ThemeMode.dark;
    _saveThemeMode();
    notifyListeners();
  }

  /// Set theme to follow system setting
  void setSystemTheme() {
    _themeMode = ThemeMode.system;
    _saveThemeMode();
    notifyListeners();
  }

  /// Toggle between light and dark themes
  void toggleTheme() {
    if (_themeMode == ThemeMode.dark) {
      setLightTheme();
    } else {
      setDarkTheme();
    }
  }

  /// Set theme mode from index
  void setThemeMode(int index) {
    if (index >= 0 && index < ThemeMode.values.length) {
      _themeMode = ThemeMode.values[index];
      _saveThemeMode();
      notifyListeners();
    }
  }
}