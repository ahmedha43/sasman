import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'storage_service.dart';

class ApiException implements Exception {
  ApiException(this.message, {this.statusCode});

  final String message;
  final int? statusCode;

  @override
  String toString() => message;
}

class ApiService {
  ApiService(this._storage);

  final StorageService _storage;

  Future<String> normalizeBaseUrl(String raw) async {
    var value = raw.trim();
    if (value.isEmpty) {
      throw ApiException('يرجى إدخال عنوان السيرفر');
    }
    if (!value.startsWith('http://') && !value.startsWith('https://')) {
      value = 'http://$value';
    }
    final uri = Uri.tryParse(value);
    if (uri == null || uri.host.isEmpty) {
      throw ApiException('عنوان السيرفر غير صالح');
    }
    while (value.endsWith('/')) {
      value = value.substring(0, value.length - 1);
    }
    return value;
  }

  Future<bool> testConnection(String rawBaseUrl) async {
    final baseUrl = await normalizeBaseUrl(rawBaseUrl);
    final uri = Uri.parse('$baseUrl/radius/api/auth/me');
    try {
      final response = await http.get(uri).timeout(const Duration(seconds: 8));
      return response.statusCode == 200 || response.statusCode == 401;
    } on TimeoutException {
      throw ApiException('انتهت مهلة الاتصال بالسيرفر');
    } catch (_) {
      throw ApiException('تعذر الوصول إلى السيرفر');
    }
  }

  Future<http.Response> get(String path) async {
    return _send('GET', path);
  }

  /// Like [get] but with a longer timeout suitable for large file downloads
  /// (backups, Excel exports, etc.).
  Future<http.Response> getFile(String path) async {
    return _send('GET', path, timeoutSeconds: 120);
  }

  Future<http.Response> post(String path, {Map<String, dynamic>? body}) async {
    return _send('POST', path, body: body);
  }

  Future<http.Response> put(String path, {Map<String, dynamic>? body}) async {
    return _send('PUT', path, body: body);
  }

  Future<http.Response> delete(String path) async {
    return _send('DELETE', path);
  }

  Future<http.Response> postMultipart(
    String path,
    String fileKey,
    List<int> bytes,
    String filename,
  ) async {
    final baseUrl = await _storage.getApiBaseUrl();
    if (baseUrl == null || baseUrl.isEmpty) {
      throw ApiException('لم يتم إعداد عنوان السيرفر');
    }

    final cookie = await _storage.getSessionCookie();
    final headers = <String, String>{
      'Accept': 'application/json',
    };
    if (cookie != null && cookie.isNotEmpty) {
      if (cookie.startsWith('Authorization=')) {
        final token = cookie.substring('Authorization='.length);
        headers['Authorization'] = token;
      } else {
        headers['Cookie'] = cookie;
      }
    }

    final uri = Uri.parse('$baseUrl$path');
    try {
      final request = http.MultipartRequest('POST', uri);
      request.headers.addAll(headers);
      request.files.add(
        http.MultipartFile.fromBytes(fileKey, bytes, filename: filename),
      );
      final streamedResponse = await request.send().timeout(const Duration(seconds: 30));
      final response = await http.Response.fromStream(streamedResponse);

      if (response.statusCode == 401) {
        throw ApiException('انتهت الجلسة، يرجى تسجيل الدخول', statusCode: 401);
      }
      if (response.statusCode >= 400) {
        throw ApiException(
          _readError(response),
          statusCode: response.statusCode,
        );
      }
      return response;
    } on TimeoutException {
      throw ApiException('انتهت مهلة الطلب');
    }
  }

  Future<http.Response> _send(
    String method,
    String path, {
    Map<String, dynamic>? body,
    int timeoutSeconds = 15,
  }) async {
    final baseUrl = await _storage.getApiBaseUrl();
    if (baseUrl == null || baseUrl.isEmpty) {
      throw ApiException('لم يتم إعداد عنوان السيرفر');
    }

    final cookie = await _storage.getSessionCookie();
    final headers = <String, String>{
      'Accept': 'application/json',
      'Content-Type': 'application/json',
    };
    if (cookie != null && cookie.isNotEmpty) {
      if (cookie.startsWith('Authorization=')) {
        final token = cookie.substring('Authorization='.length);
        headers['Authorization'] = token;
      } else {
        headers['Cookie'] = cookie;
      }
    }

    final uri = Uri.parse('$baseUrl$path');
    try {
      final requestBody = body == null ? null : jsonEncode(body);
      final request = switch (method) {
        'GET' => http.get(uri, headers: headers),
        'POST' => http.post(uri, headers: headers, body: requestBody),
        'PUT' => http.put(uri, headers: headers, body: requestBody),
        'DELETE' => http.delete(uri, headers: headers),
        _ => throw ApiException('طريقة API غير مدعومة'),
      };
      final response = await request.timeout(Duration(seconds: timeoutSeconds));

      if (response.statusCode == 401) {
        throw ApiException('انتهت الجلسة، يرجى تسجيل الدخول', statusCode: 401);
      }
      if (response.statusCode >= 400) {
        throw ApiException(
          _readError(response),
          statusCode: response.statusCode,
        );
      }
      return response;
    } on TimeoutException {
      throw ApiException('انتهت مهلة الطلب');
    }
  }

  String _readError(http.Response response) {
    try {
      final decoded = jsonDecode(response.body);
      if (decoded is Map && decoded['error'] != null) {
        return decoded['error'].toString();
      }
      if (decoded is Map && decoded['message'] != null) {
        return decoded['message'].toString();
      }
    } catch (_) {}
    return 'حدث خطأ في الطلب (${response.statusCode})';
  }
}
