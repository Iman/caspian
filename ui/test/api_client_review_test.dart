import 'dart:convert';
import 'package:caspian/api_client.dart';
import 'package:caspian/transport.dart';
import 'package:flutter_test/flutter_test.dart';

// Models the backend's advancedFlag cookie: omission retains the last choice.
class RememberingTransport implements Transport {
  bool advanced = false;
  @override
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form}) async {
    final choice = uri.queryParameters['advanced'];
    if (choice != null) advanced = choice == '1';
    return TransportResponse(
      200,
      jsonEncode({
        'view': 'dashboard',
        'ok': true,
        'page': {'Advanced': advanced},
        'strings': <String, String>{},
      }),
    );
  }

  @override
  void close() {}
}

void main() {
  test(
    'turning advanced off replaces the remembered backend preference',
    () async {
      final client = ApiClient(transport: RememberingTransport());
      addTearDown(client.close);
      final enabled = await client.request('/api/v1/state', advanced: true);
      expect(enabled['page']['Advanced'], isTrue);
      final disabled = await client.request('/api/v1/state', advanced: false);
      expect(disabled['page']['Advanced'], isFalse);
    },
  );
}
