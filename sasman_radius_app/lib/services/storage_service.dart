import 'package:shared_preferences/shared_preferences.dart';

class StorageService {
  static const _apiBaseUrlKey = 'api_base_url';
  static const _sessionCookieKey = 'session_cookie';
  static const _adminNameKey = 'admin_name';

  Future<String?> getApiBaseUrl() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_apiBaseUrlKey);
  }

  Future<void> setApiBaseUrl(String value) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_apiBaseUrlKey, value);
  }

  Future<String?> getSessionCookie() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_sessionCookieKey);
  }

  Future<void> setSessionCookie(String value) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_sessionCookieKey, value);
  }

  Future<String?> getAdminName() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_adminNameKey);
  }

  Future<void> setAdminName(String value) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_adminNameKey, value);
  }

  Future<void> clearSession() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_sessionCookieKey);
    await prefs.remove(_adminNameKey);
  }
}
