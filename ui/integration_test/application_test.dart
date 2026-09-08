// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:caspian/api_client.dart';
import 'package:caspian/app.dart';
import 'package:caspian/controller.dart';
import 'package:caspian/lifecycle.dart';
import 'package:caspian/onboarding_preferences.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

class FixtureLifecycle implements DesktopLifecycle {
  FixtureLifecycle(this.perform);
  final Future<String> Function(String) perform;
  final List<String> calls = [];

  @override
  Future<String> run(String action) {
    calls.add(action);
    return perform(action);
  }
}

// This gate models a service that has not started yet. Once released, every
// request goes over HTTP to the production Go handlers and temporary store.
class ServiceFixtureGate {
  ServiceFixtureGate._(this.server, this.upstream);
  final HttpServer server;
  final Uri upstream;
  final HttpClient client = HttpClient()..autoUncompress = false;
  bool available = false;
  int unavailableResponses = 0;
  final List<int> responseStatuses = [];
  Uri get origin => Uri.parse('http://127.0.0.1:${server.port}');

  static Future<ServiceFixtureGate> open(Uri upstream) async {
    final gate = ServiceFixtureGate._(
      await HttpServer.bind(InternetAddress.loopbackIPv4, 0),
      upstream,
    );
    gate.server.listen(gate.forward);
    return gate;
  }

  Future<void> forward(HttpRequest incoming) async {
    if (!available) {
      unavailableResponses++;
      incoming.response.statusCode = HttpStatus.serviceUnavailable;
      incoming.response.headers.contentType = ContentType.json;
      incoming.response.write('{"error":"Test service is not ready"}');
      await incoming.response.close();
      return;
    }
    try {
      final request = await client.openUrl(
        incoming.method,
        upstream.resolve(incoming.uri.toString()),
      );
      request.followRedirects = false;
      // Keep the public request host aligned with Origin for Go's CSRF guard.
      incoming.headers.forEach((name, values) {
        if (!{
          'content-length',
          'transfer-encoding',
          'connection',
        }.contains(name)) {
          request.headers.set(name, values);
        }
      });
      await request.addStream(incoming);
      final response = await request.close();
      responseStatuses.add(response.statusCode);
      incoming.response.statusCode = response.statusCode;
      response.headers.forEach((name, values) {
        if (!{
          'content-length',
          'transfer-encoding',
          'connection',
        }.contains(name)) {
          incoming.response.headers.set(name, values);
        }
      });
      await incoming.response.addStream(response);
    } finally {
      await incoming.response.close();
    }
  }

  Future<void> close() async {
    await server.close(force: true);
    client.close(force: true);
  }
}

class ObservedController extends CaspianController {
  ObservedController({required super.client, super.lifecycle});
  int submissions = 0;
  bool correctRetrySubmitted = false;

  @override
  Future<void> act(String action, Map<String, String> values) async {
    submissions++;
    correctRetrySubmitted =
        action == 'login' && values['password'] == 'correct-horse-battery';
    await super.act(action, values);
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  const endpoint = String.fromEnvironment('CASPIAN_TEST_ORIGIN');
  if (endpoint.isEmpty) {
    throw StateError(
      'Run scripts/test-integration.sh with the real Go fixture.',
    );
  }
  final origin = Uri.parse(endpoint);
  OnboardingPreferences preferences({bool completed = true}) {
    final stored = <String, bool>{};
    return OnboardingPreferences(
      read: (key) async => stored[key] ?? completed,
      write: (key, value) async {
        stored[key] = value;
      },
    );
  }

  Widget app(CaspianController c, {bool completed = true}) => CaspianApp(
    controller: c,
    onboardingPreferences: preferences(completed: completed),
  );
  Future<void> control(
    String action, [
    Map<String, dynamic> values = const {},
  ]) async {
    final http = HttpClient();
    try {
      final request = await http.postUrl(origin.resolve('/__control/$action'));
      request.headers.contentType = ContentType.json;
      request.write(jsonEncode(values));
      final response = await request.close();
      await response.drain<void>();
      expect(response.statusCode, 200);
    } finally {
      http.close(force: true);
    }
  }

  Future<void> wait(
    WidgetTester tester,
    bool Function() condition, {
    bool settle = true,
  }) async {
    final deadline = DateTime.now().add(const Duration(seconds: 20));
    while (!condition() && DateTime.now().isBefore(deadline)) {
      await tester.pump(const Duration(milliseconds: 100));
    }
    expect(
      condition(),
      isTrue,
      reason: 'Application did not reach the expected state',
    );
    if (settle) await tester.pumpAndSettle();
  }

  Future<void> tap(WidgetTester tester, String key) async {
    final target = find.byKey(ValueKey(key));
    await tester.ensureVisible(target);
    await tester.pump();
    await tester.tap(target);
    await tester.pump();
  }

  Future<CaspianController> showUnavailable(
    WidgetTester tester,
    ServiceFixtureGate gate,
    FixtureLifecycle lifecycle, {
    int attempts = 40,
  }) async {
    final c = CaspianController(
      client: ApiClient(origin: gate.origin),
      lifecycle: lifecycle,
      desktop: true,
      setupClientFactory: () => ApiClient(origin: gate.origin),
      setupReadinessAttempts: attempts,
      setupRetryDelay: const Duration(milliseconds: 100),
    );
    addTearDown(c.dispose);
    await c.refresh();
    await tester.pumpWidget(app(c, completed: false));
    await tester.pumpAndSettle();
    return c;
  }

  testWidgets(
    'native first run offers installation without a recovery detour',
    (tester) async {
      final unavailable = await HttpServer.bind(
        InternetAddress.loopbackIPv4,
        0,
      );
      unavailable.listen((request) async {
        request.response.statusCode = HttpStatus.serviceUnavailable;
        request.response.headers.contentType = ContentType.json;
        request.response.write('{"error":"Service is not installed"}');
        await request.response.close();
      });
      addTearDown(() => unavailable.close(force: true));
      final lifecycle = FixtureLifecycle(
        (_) async => 'The service action completed.',
      );
      final c = CaspianController(
        client: ApiClient(
          origin: Uri.parse('http://127.0.0.1:${unavailable.port}'),
        ),
        lifecycle: lifecycle,
        desktop: true,
      );
      addTearDown(c.dispose);
      await c.refresh();
      await tester.pumpWidget(app(c, completed: false));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('onboarding-install')), findsOneWidget);
      expect(
        lifecycle.calls,
        isEmpty,
        reason: 'Installing requires an explicit action',
      );
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'native guided install waits for the real API then acknowledges its password',
    (tester) async {
      await control('reset');
      final gate = await ServiceFixtureGate.open(origin);
      addTearDown(gate.close);
      final installed = Completer<String>();
      final lifecycle = FixtureLifecycle((_) => installed.future);
      final c = await showUnavailable(tester, gate, lifecycle);
      await tap(tester, 'onboarding-install');
      expect(lifecycle.calls, ['install']);
      expect(c.setupPhase, ServiceSetupPhase.installing);
      expect(find.byKey(const ValueKey('login-password')), findsNothing);
      installed.complete('first-run panel password: correct-horse-battery');
      await wait(
        tester,
        () =>
            c.setupPhase == ServiceSetupPhase.checking &&
            gate.unavailableResponses >= 2,
        settle: false,
      );
      expect(c.busy, true);
      expect(find.byKey(const ValueKey('onboarding-continue')), findsNothing);
      gate.available = true;
      await wait(
        tester,
        () => !c.busy && c.setupPhase == ServiceSetupPhase.ready,
      );
      expect(c.state['view'], 'login');
      expect(find.byKey(const ValueKey('onboarding-password')), findsOneWidget);
      expect(
        tester
            .widget<FilledButton>(
              find.byKey(const ValueKey('onboarding-continue')),
            )
            .onPressed,
        isNull,
      );
      await tap(tester, 'onboarding-saved');
      await tap(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(c.setupPassword, isNull);
      expect(c.setupPasswordAcknowledged, true);
      await tester.enterText(
        find.byKey(const ValueKey('login-password')),
        'correct-horse-battery',
      );
      await tap(tester, 'login-submit');
      await wait(tester, () => !c.busy);
      expect(
        c.state['view'],
        'dashboard',
        reason:
            'Forwarded HTTP status codes: ${gate.responseStatuses}; API error: ${c.error != null}',
      );
      expect(lifecycle.calls, ['install']);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'native guided install failure stays retryable without claiming ready',
    (tester) async {
      await control('reset');
      final gate = await ServiceFixtureGate.open(origin);
      addTearDown(gate.close);
      var attempts = 0;
      final lifecycle = FixtureLifecycle((_) async {
        if (++attempts == 1) {
          throw StateError('Simulated approval or installer failure');
        }
        gate.available = true;
        return 'The service action completed.';
      });
      final c = await showUnavailable(tester, gate, lifecycle);
      await tap(tester, 'onboarding-install');
      await wait(
        tester,
        () => !c.busy && c.setupPhase == ServiceSetupPhase.failed,
      );
      expect(find.byKey(const ValueKey('onboarding-retry')), findsOneWidget);
      expect(find.byKey(const ValueKey('onboarding-continue')), findsNothing);
      expect(c.state, isEmpty);
      expect(c.setupError, isNotEmpty);
      await tap(tester, 'onboarding-retry');
      await wait(
        tester,
        () => !c.busy && c.setupPhase == ServiceSetupPhase.ready,
      );
      expect(lifecycle.calls, ['install', 'install']);
      expect(c.state['view'], 'login');
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'native ready service welcome stays completed when the app reopens',
    (tester) async {
      await control('reset');
      final memory = preferences(completed: false);
      final lifecycle = FixtureLifecycle(
        (_) async =>
            throw StateError('Ready service must not invoke an installer'),
      );
      final first = CaspianController(
        client: ApiClient(origin: origin),
        lifecycle: lifecycle,
        desktop: true,
      );
      addTearDown(first.dispose);
      await first.refresh();
      await tester.pumpWidget(
        CaspianApp(controller: first, onboardingPreferences: memory),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('onboarding-continue')), findsOneWidget);
      final saved = find.byKey(const ValueKey('onboarding-saved'));
      if (saved.evaluate().isNotEmpty) await tap(tester, 'onboarding-saved');
      await tap(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
      expect(await memory.welcomeCompleted(), isTrue);
      await tester.pumpWidget(const SizedBox());

      final reopened = CaspianController(
        client: ApiClient(origin: origin),
        lifecycle: lifecycle,
        desktop: true,
      );
      addTearDown(reopened.dispose);
      await reopened.refresh();
      await tester.pumpWidget(
        CaspianApp(controller: reopened, onboardingPreferences: memory),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('onboarding-continue')), findsNothing);
      expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
      expect(lifecycle.calls, isEmpty);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'native successful helper with unavailable API does not claim setup completed',
    (tester) async {
      await control('reset');
      final gate = await ServiceFixtureGate.open(origin);
      addTearDown(gate.close);
      final lifecycle = FixtureLifecycle(
        (_) async => 'The service action completed.',
      );
      final c = await showUnavailable(tester, gate, lifecycle, attempts: 2);
      await tap(tester, 'onboarding-install');
      await wait(
        tester,
        () => !c.busy && c.setupPhase == ServiceSetupPhase.failed,
      );
      expect(gate.unavailableResponses, greaterThanOrEqualTo(3));
      expect(find.byKey(const ValueKey('onboarding-continue')), findsNothing);
      expect(find.byKey(const ValueKey('onboarding-retry')), findsOneWidget);
      expect(lifecycle.calls, ['install']);
      gate.available = true;
      await tap(tester, 'onboarding-retry');
      await wait(
        tester,
        () => !c.busy && c.setupPhase == ServiceSetupPhase.ready,
      );
      expect(c.state['view'], 'login');
      expect(lifecycle.calls, [
        'install',
      ], reason: 'Readiness retry must not reinstall services');
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'native app rejects wrong login then connects, cuts and restores through Go',
    (tester) async {
      await control('reset');
      final lifecycle = FixtureLifecycle(
        (_) async =>
            throw StateError('Existing login must not run installation'),
      );
      final c = ObservedController(
        client: ApiClient(origin: origin),
        lifecycle: lifecycle,
      );
      addTearDown(c.dispose);
      await c.refresh();
      await tester.pumpWidget(app(c));
      await tester.pumpAndSettle();
      expect(c.state['view'], 'login');
      await tester.enterText(
        find.byKey(const ValueKey('login-password')),
        'wrong-password',
      );
      await tester.tap(find.byKey(const ValueKey('login-submit')));
      await wait(tester, () => !c.busy);
      expect(c.state['view'], 'login');
      expect(c.state['ok'], false);
      expect(c.submissions, 1);
      await tester.ensureVisible(find.byKey(const ValueKey('login-password')));
      await tester.tap(find.byKey(const ValueKey('login-password')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('login-password')),
        'correct-horse-battery',
      );
      bool hasRetryText() =>
          tester
              .widget<TextField>(find.byKey(const ValueKey('login-password')))
              .controller!
              .text ==
          'correct-horse-battery';
      expect(
        hasRetryText(),
        isTrue,
        reason: 'Text entry must update the field',
      );
      await tester.pumpAndSettle();
      expect(
        hasRetryText(),
        isTrue,
        reason: 'Text must survive settling the platform input',
      );
      await tester.ensureVisible(find.byKey(const ValueKey('login-submit')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('login-submit')));
      expect(c.submissions, 2, reason: 'Retry tap must submit the form');
      expect(
        c.correctRetrySubmitted,
        isTrue,
        reason: 'Retry must submit the updated field value',
      );
      await wait(tester, () => !c.busy);
      expect(
        c.state['view'],
        'dashboard',
        reason:
            '${c.error ?? c.state["page"]?["ProblemHeadline"] ?? "No problem reported"}',
      );
      Map<String, dynamic> page() => c.state['page'] as Map<String, dynamic>;
      if (page()['Running'] == true) {
        await tester.tap(find.byKey(const ValueKey('power')));
        await wait(tester, () => !c.busy && page()['Running'] == false);
      }
      await tester.tap(find.byKey(const ValueKey('power')));
      await wait(tester, () => !c.busy && page()['Running'] == true);
      expect(page()['Connected'], true);
      await tester.tap(find.byKey(const ValueKey('traffic')));
      await wait(tester, () => !c.busy && page()['TrafficCut'] == true);
      expect(page()['Connected'], false);
      await tester.tap(find.byKey(const ValueKey('traffic')));
      await wait(tester, () => !c.busy && page()['TrafficCut'] == false);
      expect(page()['Connected'], true);
      expect(lifecycle.calls, isEmpty);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'native app shows a privileged start refusal without claiming connection',
    (tester) async {
      await control('reset');
      await control('state', {
        'engine': 'stopped',
        'hotspot': false,
        'start_fault': true,
      });
      final c = CaspianController(client: ApiClient(origin: origin));
      addTearDown(c.dispose);
      await c.refresh();
      await tester.pumpWidget(app(c));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('login-password')),
        'correct-horse-battery',
      );
      await tester.tap(find.byKey(const ValueKey('login-submit')));
      await wait(tester, () => !c.busy && c.state['view'] == 'dashboard');
      await tester.tap(find.byKey(const ValueKey('power')));
      await wait(tester, () => !c.busy);
      expect(c.state['ok'], false);
      expect(c.state['page']['HasProblem'], true);
      expect(c.state['page']['Connected'], false);
      await tester.pumpWidget(const SizedBox());
    },
  );
}
