import 'dart:convert';

import 'api_service.dart';
import 'storage_service.dart';

class AuthService {
  AuthService(this._api, this._storage);

  final ApiService _api;
  final StorageService _storage;

  String _normalizeSetCookie(String rawCookie) {
    final trimmed = rawCookie.trim();
    if (trimmed.isEmpty) return trimmed;
    final firstCookie = trimmed.split(',').first;
    return firstCookie.split(';').first.trim();
  }

  Future<void> login(String username, String password) async {
    final response = await _api.post(
      '/radius/api/auth/login',
      body: {'username': username.trim(), 'password': password},
    );

    final setCookie = response.headers['set-cookie'];
    if (setCookie != null && setCookie.isNotEmpty) {
      final cookieValue = _normalizeSetCookie(setCookie);
      if (cookieValue.isNotEmpty) {
        await _storage.setSessionCookie(cookieValue);
      }
    }

    try {
      final decoded = jsonDecode(response.body);
      if (decoded is Map && decoded['token'] != null) {
        await _storage.setSessionCookie('Authorization=Bearer ${decoded['token']}');
      }
      if (decoded is Map && decoded['username'] != null) {
        await _storage.setAdminName(decoded['username'].toString());
      } else {
        await _storage.setAdminName(username.trim());
      }
    } catch (_) {
      await _storage.setAdminName(username.trim());
    }
  }

  Future<void> logout() async {
    try {
      await _api.post('/radius/api/auth/logout');
    } catch (_) {}
    await _storage.clearSession();
  }
}
