// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:convert';
import 'dart:io';
import 'transport.dart';

class PlatformTransport implements Transport {
  final HttpClient _client = HttpClient();
  final Map<String, Cookie> _cookies = {};
  Uri? _origin;

  @override
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form}) async {
    // A session belongs to exactly one origin and never follows redirects.
    _origin ??= Uri.parse(uri.origin);
    if (uri.origin != _origin!.origin) throw StateError('Origin changed');
    final request = await _client.openUrl(form == null ? 'GET' : 'POST', uri);
    request.followRedirects = false;
    request.headers.set(HttpHeaders.acceptHeader, 'application/json');
    request.cookies.addAll(_cookies.values);
    if (form != null) {
      request.headers.set(
        HttpHeaders.contentTypeHeader,
        'application/x-www-form-urlencoded',
      );
      request.headers.set('Origin', uri.origin);
      request.write(Uri(queryParameters: form).query);
    }
    final response = await request.close();
    for (final cookie in response.cookies) {
      if (cookie.maxAge == 0 || cookie.value.isEmpty) {
        _cookies.remove(cookie.name);
      } else {
        _cookies[cookie.name] = cookie;
      }
    }
    final body = await response.transform(utf8.decoder).join();
    return TransportResponse(response.statusCode, body);
  }

  @override
  void close() {
    _cookies.clear();
    _client.close(force: true);
  }
}
