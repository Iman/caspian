// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:async';
import 'dart:convert';
import 'package:caspian/api_client.dart';
import 'package:caspian/controller.dart';
import 'package:caspian/lifecycle.dart';
import 'package:caspian/transport.dart';
import 'package:flutter_test/flutter_test.dart';

class SetupTransport implements Transport {
  int calls = 0;
  bool closed = false;
  Future<TransportResponse> Function(int)? respond;
  @override
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form}) async {
    calls++;
    return respond != null
        ? respond!(calls)
        : reply('login', token: 'probe-token');
  }

  @override
  void close() => closed = true;
}

TransportResponse reply(String view, {String token = 'token'}) =>
    TransportResponse(
      200,
      jsonEncode({
        'view': view,
        'ok': true,
        'page': {'CSRF': token},
        'strings': {},
      }),
    );

class SetupLifecycle implements DesktopLifecycle {
  final actions = <String>[];
  Future<String> Function()? respond;
  @override
  Future<String> run(String action) async {
    actions.add(action);
    return respond != null ? respond!() : 'The service action completed.';
  }
}

void main() {
  late SetupTransport initial;
  late SetupLifecycle lifecycle;
  late CaspianController c;
  final probes = <SetupTransport>[];
  setUp(() {
    initial = SetupTransport();
    lifecycle = SetupLifecycle();
    probes.clear();
    c = CaspianController(
      desktop: true,
      client: ApiClient(transport: initial),
      lifecycle: lifecycle,
      setupClientFactory: () {
        final probe = SetupTransport();
        probes.add(probe);
        return ApiClient(transport: probe);
      },
      setupRetryDelay: Duration.zero,
      setupRequestTimeout: const Duration(milliseconds: 20),
      setupReadinessAttempts: 3,
    );
  });
  tearDown(() => c.dispose());

  test(
    'setup requires an explicit action and exposes observed phases',
    () async {
      expect(lifecycle.actions, isEmpty);
      expect(c.setupPhase, ServiceSetupPhase.idle);
      final phases = <ServiceSetupPhase>[];
      c.addListener(() => phases.add(c.setupPhase));
      await c.setUpService('install');
      expect(lifecycle.actions, ['install']);
      expect(
        phases,
        containsAllInOrder([
          ServiceSetupPhase.installing,
          ServiceSetupPhase.checking,
          ServiceSetupPhase.ready,
        ]),
      );
      expect(c.state['view'], 'login');
      expect(c.setupPassword, isNull);
      expect(initial.closed, true);
      expect(probes.single.closed, false);
      await c.act('login', {'password': 'fictional-password'});
      expect(probes.single.calls, 2);
    },
  );

  test(
    'delayed readiness retries until one valid authentication view',
    () async {
      var attempts = 0;
      c.dispose();
      final probe = SetupTransport()
        ..respond = (_) async {
          attempts++;
          if (attempts < 3) throw StateError('offline');
          return reply('setup');
        };
      c = CaspianController(
        desktop: true,
        lifecycle: lifecycle,
        client: ApiClient(transport: initial),
        setupClientFactory: () => ApiClient(transport: probe),
        setupRetryDelay: Duration.zero,
        setupReadinessAttempts: 3,
      );
      await c.setUpService('start');
      expect(attempts, 3);
      expect(c.setupPhase, ServiceSetupPhase.ready);
      expect(c.state['view'], 'setup');
    },
  );

  test(
    'readiness failure retains issued password through retry until acknowledged',
    () async {
      lifecycle.respond = () async =>
          'first-run panel password: fictional-issued-password';
      c.dispose();
      var recovered = false;
      c = CaspianController(
        desktop: true,
        lifecycle: lifecycle,
        client: ApiClient(transport: initial),
        setupClientFactory: () {
          final probe = SetupTransport()
            ..respond = (_) async {
              if (!recovered) throw StateError('private diagnostic');
              return reply('login');
            };
          probes.add(probe);
          return ApiClient(transport: probe);
        },
        setupRetryDelay: Duration.zero,
        setupReadinessAttempts: 2,
      );
      await c.setUpService('install');
      expect(c.setupPhase, ServiceSetupPhase.failed);
      expect(probes.single.calls, 2);
      expect(probes.single.closed, true);
      expect(c.setupPassword, 'fictional-issued-password');
      expect(c.setupError, isNot(contains('private')));
      recovered = true;
      lifecycle.respond = null;
      expect(c.setupNeedsReadinessRetry, true);
      await c.retrySetupCheck();
      expect(lifecycle.actions, ['install']);
      expect(c.setupPhase, ServiceSetupPhase.ready);
      expect(c.setupNeedsReadinessRetry, false);
      expect(c.setupPassword, 'fictional-issued-password');
      expect(c.setupPasswordAcknowledged, false);
      c.acknowledgeSetupPassword();
      expect(c.setupPassword, isNull);
      expect(c.setupPasswordAcknowledged, true);
    },
  );

  test(
    'installer failure is factual and retry does not assume cancellation',
    () async {
      lifecycle.respond = () async => throw StateError('private diagnostic');
      await c.setUpService('install');
      expect(c.setupPhase, ServiceSetupPhase.failed);
      expect(c.setupNeedsReadinessRetry, false);
      await c.retrySetupCheck();
      expect(lifecycle.actions, ['install']);
      expect(c.setupError, contains('did not complete'));
      expect(c.setupError!.toLowerCase(), isNot(contains('cancelled')));
      expect(probes, isEmpty);
      lifecycle.respond = null;
      await c.setUpService('install');
      expect(c.setupPhase, ServiceSetupPhase.ready);
    },
  );

  test(
    'timed out probe closes and cannot replace current authentication state',
    () async {
      c.dispose();
      final pending = Completer<TransportResponse>();
      final probe = SetupTransport()..respond = (_) => pending.future;
      c = CaspianController(
        desktop: true,
        lifecycle: lifecycle,
        client: ApiClient(transport: initial),
        setupClientFactory: () => ApiClient(transport: probe),
        setupRequestTimeout: const Duration(milliseconds: 5),
      );
      await c.setUpService('install');
      expect(c.setupPhase, ServiceSetupPhase.failed);
      expect(probe.calls, 1);
      expect(probe.closed, true);
      expect(c.busy, false);
      pending.complete(reply('dashboard'));
      await Future<void>.delayed(Duration.zero);
      expect(c.state, isEmpty);
      expect(c.setupPhase, ServiceSetupPhase.failed);
    },
  );

  test('malformed readiness replies cannot complete setup', () async {
    c.dispose();
    final probe = SetupTransport()..respond = (_) async => reply('unknown');
    c = CaspianController(
      desktop: true,
      lifecycle: lifecycle,
      client: ApiClient(transport: initial),
      setupClientFactory: () => ApiClient(transport: probe),
      setupReadinessAttempts: 1,
    );
    await c.setUpService('start');
    expect(c.setupPhase, ServiceSetupPhase.failed);
    expect(probe.closed, true);
  });

  test(
    'successful recovery replaces the unacknowledged setup password',
    () async {
      lifecycle.respond = () async =>
          'first-run panel password: fictional-original';
      await c.setUpService('install');
      lifecycle.respond = () async =>
          'New Caspian panel password: fictional-replacement';
      await c.serviceAction('reset-password');
      expect(c.setupPassword, 'fictional-replacement');
      expect(c.setupPasswordAcknowledged, false);
    },
  );

  test('external recovery removes the obsolete setup password', () async {
    lifecycle.respond = () async =>
        'first-run panel password: fictional-original';
    await c.setUpService('install');
    lifecycle.respond = () async =>
        'The new password was displayed in the administrator window.';
    await c.serviceAction('reset-password');
    expect(c.setupPassword, isNull);
    expect(c.setupPasswordAcknowledged, false);
  });

  test(
    'failed recovery and unrelated service actions retain the setup password',
    () async {
      lifecycle.respond = () async =>
          'first-run panel password: fictional-original';
      await c.setUpService('install');
      lifecycle.respond = () async => throw StateError('synthetic failure');
      await c.serviceAction('reset-password');
      expect(c.setupPassword, 'fictional-original');
      lifecycle.respond = null;
      await c.serviceAction('start');
      expect(c.setupPassword, 'fictional-original');
    },
  );

  test('web and overlapping actions never invoke another installer', () async {
    await expectLater(c.setUpService('reset-password'), throwsArgumentError);
    final pending = Completer<String>();
    lifecycle.respond = () => pending.future;
    final first = c.setUpService('install');
    await c.setUpService('start');
    expect(lifecycle.actions, ['install']);
    pending.complete('The service action completed.');
    await first;
    final web = CaspianController(
      desktop: false,
      lifecycle: lifecycle,
      client: ApiClient(transport: SetupTransport()),
    );
    await web.setUpService('install');
    expect(lifecycle.actions, ['install']);
    web.dispose();
  });

  testWidgets('automatic polling leaves a ready login token unchanged', (
    tester,
  ) async {
    initial.respond = (_) async => throw StateError('offline');
    final probe = SetupTransport();
    final polling = CaspianController(
      desktop: true,
      lifecycle: lifecycle,
      client: ApiClient(transport: initial),
      setupClientFactory: () => ApiClient(transport: probe),
    );
    polling.start();
    await tester.pump();
    final setup = polling.setUpService('install');
    await tester.pump();
    await setup;
    expect(polling.setupPhase, ServiceSetupPhase.ready);
    expect(polling.state['page']['CSRF'], 'probe-token');
    await tester.pump(const Duration(seconds: 15));
    expect(probe.calls, 1);
    expect(polling.state['page']['CSRF'], 'probe-token');
    polling.dispose();
  });
}
