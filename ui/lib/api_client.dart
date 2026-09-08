// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'transport.dart';

class ApiSessionExpired implements Exception {}

class ApiClient {
  ApiClient({Uri? origin, Transport? transport})
    : origin =
          origin ??
          (kIsWeb
              ? Uri.parse(Uri.base.origin)
              : Uri.parse('http://127.0.0.1:8088')),
      _transport = transport ?? Transport();
  final Uri origin;
  final Transport _transport;

  Future<Map<String, dynamic>> request(
    String path, {
    Map<String, String>? form,
    String language = 'en',
    bool advanced = false,
  }) async {
    final uri = origin.replace(
      path: path,
      queryParameters: {'lang': language, 'advanced': advanced ? '1' : '0'},
    );
    final result = await _transport
        .send(uri, form: form)
        .timeout(const Duration(seconds: 100));
    final decoded = jsonDecode(result.body);
    if (decoded is! Map<String, dynamic>) {
      throw const FormatException('Invalid reply');
    }
    // Validation failures carry the same page envelope, with the server's
    // translated problem. Keep that page instead of hiding it behind a toast.
    if (decoded['view'] is String &&
        decoded['page'] is Map &&
        decoded['strings'] is Map) {
      return decoded;
    }
    if (result.status == 401) throw ApiSessionExpired();
    if (result.status < 200 || result.status >= 300) {
      throw const FormatException('Request refused');
    }
    return decoded;
  }

  void close() => _transport.close();
}
