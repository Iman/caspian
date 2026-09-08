// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:async';
import 'dart:convert';
import 'package:caspian/api_client.dart';
import 'package:caspian/controller.dart';
import 'package:caspian/lifecycle.dart';
import 'package:caspian/transport.dart';
import 'package:flutter_test/flutter_test.dart';

Map<String, dynamic> page(String view, {bool ok = true}) => {
  'view': view,
  'ok': ok,
  'page': {'CSRF': 'test-token', 'Lang': 'en'},
  'strings': <String, String>{},
};

class FakeTransport implements Transport {
  final calls = <({Uri uri, Map<String, String>? form})>[];
  Future<TransportResponse> Function(Uri, Map<String, String>?)? respond;
  bool closed = false;
  @override
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form}) async {
    calls.add((uri: uri, form: form));
    return respond != null
        ? respond!(uri, form)
        : TransportResponse(200, jsonEncode(page('dashboard')));
  }

  @override
  void close() {
    closed = true;
  }
}

class FakeLifecycle implements DesktopLifecycle {
  final actions = <String>[];
  bool fail = false;
  @override
  Future<String> run(String action) async {
    actions.add(action);
    if (fail) throw StateError('private diagnostic');
    return 'Service action completed.';
  }
}

void main() {
  late FakeTransport transport;
  late FakeLifecycle lifecycle;
  late CaspianController c;
  setUp(() {
    transport = FakeTransport();
    lifecycle = FakeLifecycle();
    c = CaspianController(
      client: ApiClient(transport: transport),
      lifecycle: lifecycle,
      desktop: true,
    );
  });
  tearDown(() => c.dispose());
  test(
    'read-only refresh keeps the request lock and clears its marker on failure',
    () async {
      final reply = Completer<TransportResponse>();
      transport.respond = (_, _) => reply.future;
      final pending = c.refresh();
      expect(c.refreshing, isTrue);
      expect(c.busy, isTrue);
      await c.act('login', {'password': 'fictional-test-password'});
      expect(transport.calls.length, 1);
      reply.completeError(StateError('offline'));
      await pending;
      expect(c.refreshing, isFalse);
      expect(c.busy, isFalse);
      expect(c.error, contains('Cannot reach'));
      transport.respond = null;
      await c.refresh();
      expect(c.refreshing, isFalse);
      expect(c.error, isNull);
    },
  );
  test(
    'setup and login send only explicit form and current CSRF then accept dashboard',
    () async {
      await c.refresh();
      await c.act('login', {
        'password': 'fictional-test-password',
        'csrf': 'caller-cannot-override',
      });
      expect(transport.calls.last.form, {
        'password': 'fictional-test-password',
        'csrf': 'test-token',
      });
      expect(transport.calls.last.uri.path, '/api/v1/login');
      expect(c.state['view'], 'dashboard');
      expect(c.error, isNull);
    },
  );
  test(
    'rejected configuration keeps translated problem envelope visible',
    () async {
      transport.respond = (_, _) async =>
          TransportResponse(422, jsonEncode(page('dashboard', ok: false)));
      await c.act('config', {'config': 'invalid'});
      expect(c.state['ok'], false);
      expect(c.error, isNull);
      expect(c.busy, false);
    },
  );
  test(
    'lost backend clears stale connected state and secrets without showing exception',
    () async {
      await c.refresh();
      transport.respond = (_, _) async =>
          throw StateError('private diagnostic');
      await c.refresh();
      expect(c.state, isEmpty);
      expect(c.error, contains('Cannot reach'));
      expect(c.error, isNot(contains('private')));
      transport.respond = null;
      await c.refresh();
      expect(c.error, isNull);
    },
  );
  test(
    'duplicate mutation and polling cannot overtake an action in flight',
    () async {
      final reply = Completer<TransportResponse>();
      transport.respond = (_, _) => reply.future;
      final pending = c.act('power', {'on': '1'});
      expect(c.refreshing, isFalse);
      await c.act('power', {'on': '1'});
      await c.refresh();
      expect(transport.calls.length, 1);
      reply.complete(TransportResponse(200, jsonEncode(page('dashboard'))));
      await pending;
      expect(c.busy, false);
    },
  );
  test(
    'locale and advanced are sent on requests and unsupported actions rejected',
    () async {
      await c.setLanguage('fa');
      await c.setAdvanced(true);
      expect(transport.calls.last.uri.queryParameters, {
        'lang': 'fa',
        'advanced': '1',
      });
      await c.setLanguage('en');
      expect(c.language, 'en');
      await c.setLanguage('not-a-language');
      expect(c.language, 'en');
      await expectLater(c.act('../admin', {}), throwsArgumentError);
      transport.respond = (_, _) async => throw StateError('offline');
      await c.setLanguage('fa');
      expect(c.error, contains('کاسپین'));
    },
  );
  test(
    'recovery is independent of API readiness and failures stay visible',
    () async {
      transport.respond = (_, _) async => throw StateError('offline');
      await c.recoverService();
      expect(lifecycle.actions, ['start']);
      expect(c.serviceMessage, 'Service action completed.');
      lifecycle.fail = true;
      await c.serviceAction('restart');
      expect(c.serviceMessage, contains('did not complete'));
      expect(c.supportedServiceActions, contains('reset-password'));
    },
  );
  test('identifiers use authenticated dedicated endpoint', () async {
    transport.respond = (_, _) async =>
        const TransportResponse(200, '{"uuid":"synthetic"}');
    expect(await c.identifiers(), {'uuid': 'synthetic'});
    expect(transport.calls.single.uri.path, '/identifiers.json');
  });
  test(
    'expired session returns login instead of claiming backend unavailable',
    () async {
      var mutation = false;
      transport.respond = (uri, form) async {
        if (form != null) {
          mutation = true;
          return const TransportResponse(401, '{"error":"not signed in"}');
        }
        return TransportResponse(
          200,
          jsonEncode(page(mutation ? 'login' : 'dashboard')),
        );
      };
      await c.refresh();
      await c.act('power', {'on': '1'});
      expect(c.state['view'], 'login');
      expect(c.error, isNull);
      expect(transport.calls.where((r) => r.form != null).length, 1);
    },
  );
}
