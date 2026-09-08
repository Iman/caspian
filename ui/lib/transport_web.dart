// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:http/browser_client.dart';
import 'transport.dart';

class PlatformTransport implements Transport {
  final BrowserClient _client = BrowserClient()..withCredentials = true;
  @override
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form}) async {
    if (uri.origin != Uri.base.origin) throw StateError('Origin changed');
    final response = form == null
        ? await _client.get(uri, headers: {'Accept': 'application/json'})
        : await _client.post(
            uri,
            headers: {'Accept': 'application/json'},
            body: form,
          );
    return TransportResponse(response.statusCode, response.body);
  }

  @override
  void close() => _client.close();
}
