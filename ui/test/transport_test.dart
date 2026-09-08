// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:io';
import 'package:caspian/transport.dart';
import 'package:caspian/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'native session persists HttpOnly cookies, sends CSRF form, clears logout cookie',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      final transport = Transport();
      addTearDown(() async {
        transport.close();
        await server.close(force: true);
      });
      final base = Uri.parse('http://127.0.0.1:${server.port}');
      final requests = <HttpRequest>[];
      server.listen((request) async {
        requests.add(request);
        if (request.uri.path == '/login') {
          request.response.cookies.add(
            Cookie('session', 'test-session')
              ..httpOnly = true
              ..path = '/',
          );
        } else if (request.uri.path == '/logout') {
          request.response.cookies.add(
            Cookie('session', '')
              ..maxAge = 0
              ..path = '/',
          );
        }
        if (request.method == 'POST') {
          expect(request.headers.value('Origin'), base.origin);
          expect(
            request.headers.contentType?.mimeType,
            'application/x-www-form-urlencoded',
          );
        }
        request.response.write('{"ok":true}');
        await request.response.close();
      });
      expect((await transport.send(base.resolve('/login'))).status, 200);
      await transport.send(
        base.resolve('/power'),
        form: {'csrf': 'token', 'on': '1'},
      );
      expect(requests[1].cookies.single.value, 'test-session');
      await transport.send(base.resolve('/logout'));
      await transport.send(base.resolve('/state'));
      expect(requests.last.cookies, isEmpty);
      await expectLater(
        transport.send(Uri.parse('http://localhost:${server.port}/state')),
        throwsStateError,
      );
    },
  );
  test(
    'native transport refuses redirects so credentials never follow another host',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      final transport = Transport();
      addTearDown(() async {
        transport.close();
        await server.close(force: true);
      });
      server.listen((r) async {
        r.response.statusCode = 302;
        r.response.headers.set('Location', 'http://example.invalid/');
        await r.response.close();
      });
      expect(
        (await transport.send(
          Uri.parse('http://127.0.0.1:${server.port}/'),
        )).status,
        302,
      );
    },
  );
  test('malformed and non-envelope refusal replies fail closed', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final client = ApiClient(
      origin: Uri.parse('http://127.0.0.1:${server.port}'),
    );
    addTearDown(() async {
      client.close();
      await server.close(force: true);
    });
    server.listen((r) async {
      switch (r.uri.path) {
        case '/array':
          r.response.write('[]');
        case '/refused':
          r.response.statusCode = 403;
          r.response.write('{"error":"forbidden"}');
        default:
          r.response.write('not-json');
      }
      await r.response.close();
    });
    for (final path in ['/array', '/refused', '/malformed']) {
      await expectLater(client.request(path), throwsFormatException);
    }
  });
}
