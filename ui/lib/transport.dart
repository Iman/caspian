// SPDX-License-Identifier: AGPL-3.0-or-later
import 'transport_io.dart'
    if (dart.library.js_interop) 'transport_web.dart'
    as platform;

class TransportResponse {
  const TransportResponse(this.status, this.body);
  final int status;
  final String body;
}

abstract class Transport {
  factory Transport() = platform.PlatformTransport;
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form});
  void close();
}
