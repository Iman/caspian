// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:caspian/app.dart';
import 'package:caspian/controller.dart';
import 'package:caspian/onboarding.dart';
import 'package:caspian/onboarding_preferences.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'app_test.dart' show FakeController, fixture;

class SetupController extends FakeController {
  SetupController({super.desktop = true});
  final setupActions = <String>[];
  bool readinessRetry = false;
  int readinessChecks = 0;
  @override
  bool get setupNeedsReadinessRetry => readinessRetry;
  @override
  Future<void> retrySetupCheck() async {
    readinessChecks++;
    publish(ServiceSetupPhase.checking, password: setupPassword);
  }

  @override
  Future<void> setUpService(String action) async {
    setupActions.add(action);
    setupPhase = ServiceSetupPhase.installing;
    busy = true;
    notifyListeners();
  }

  void publish(
    ServiceSetupPhase phase, {
    String? password,
    String? problem,
    String? view,
  }) {
    setupPhase = phase;
    setupPassword = password;
    setupError = problem;
    busy =
        phase == ServiceSetupPhase.installing ||
        phase == ServiceSetupPhase.checking;
    if (view != null) state = fixture(view: view, fa: language == 'fa');
    notifyListeners();
  }
}

Future<SetupController> showSetup(
  WidgetTester tester, {
  TargetPlatform platform = TargetPlatform.linux,
  bool fa = false,
  bool desktop = true,
  Size size = const Size(1000, 1100),
  OnboardingPreferences? preferences,
}) async {
  debugDefaultTargetPlatformOverride = platform;
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final c = SetupController(desktop: desktop)..language = fa ? 'fa' : 'en';
  addTearDown(c.dispose);
  await tester.pumpWidget(
    RepaintBoundary(
      key: const ValueKey('first-run-screenshot'),
      child: CaspianApp(
        controller: c,
        onboardingPreferences:
            preferences ??
            OnboardingPreferences(
              read: (_) async => false,
              write: (_, _) async {},
            ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return c;
}

Future<void> press(WidgetTester tester, String id) async {
  final control = find.byKey(ValueKey(id));
  await tester.ensureVisible(control);
  await tester.tap(control);
  await tester.pump();
}

void onboardingTestWidgets(
  String name,
  Future<void> Function(WidgetTester) body,
) {
  testWidgets(name, (tester) async {
    try {
      await body(tester);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() async {
    for (final font in {
      'Vazirmatn': 'assets/fonts/Vazirmatn.ttf',
      'CaspianMono': 'assets/fonts/NotoSansMono.ttf',
      'MaterialIcons': 'fonts/MaterialIcons-Regular.otf',
    }.entries) {
      await (FontLoader(font.key)..addFont(rootBundle.load(font.value))).load();
    }
  });
  onboardingTestWidgets(
    'unavailable desktop offers visible setup and existing-service start',
    (tester) async {
      final c = FakeController(desktop: true)..state = {};
      addTearDown(c.dispose);
      await tester.pumpWidget(
        CaspianApp(
          controller: c,
          onboardingPreferences: OnboardingPreferences(
            read: (_) async => false,
            write: (_, _) async {},
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Set up Caspian'), findsOneWidget);
      expect(find.byKey(const ValueKey('onboarding-install')), findsOneWidget);
      expect(find.byKey(const ValueKey('onboarding-start')), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'first sign in offers configuration and hotspot setup checklist',
    (tester) async {
      final c = FakeController()..state = fixture();
      (c.state['page'] as Map<String, dynamic>).addAll({
        'SetupIncomplete': true,
        'HasConfig': false,
        'HotspotReady': false,
        'Running': false,
        'Connected': false,
        'DeviceCount': 0,
      });
      addTearDown(c.dispose);
      await tester.pumpWidget(
        CaspianApp(
          controller: c,
          onboardingPreferences: OnboardingPreferences(
            read: (_) async => false,
            write: (_, _) async {},
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-checklist')), findsOneWidget);
      expect(find.byKey(const ValueKey('checklist-config')), findsOneWidget);
      expect(find.byKey(const ValueKey('checklist-hotspot')), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'Mac install waits for readiness and saved password before sign in',
    (tester) async {
      final c = await showSetup(tester, platform: TargetPlatform.macOS);
      expect(
        find.textContaining('Copy Caspian to Applications'),
        findsOneWidget,
      );
      expect(find.textContaining('Mac administrator password'), findsOneWidget);
      await press(tester, 'onboarding-install');
      expect(c.setupActions, ['install']);
      expect(
        find.textContaining('Waiting for system approval'),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('onboarding-continue')), findsNothing);
      c.publish(ServiceSetupPhase.checking);
      await tester.pump();
      expect(
        find.textContaining('Waiting for Caspian to respond.'),
        findsOneWidget,
      );
      c.publish(
        ServiceSetupPhase.ready,
        password: 'example-generated-password',
        view: 'login',
      );
      await tester.pumpAndSettle();
      expect(find.text('example-generated-password'), findsOneWidget);
      expect(
        tester
            .widget<FilledButton>(
              find.byKey(const ValueKey('onboarding-continue')),
            )
            .onPressed,
        isNull,
      );
      await press(tester, 'onboarding-saved');
      await press(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(c.setupPassword, isNull);
      expect(c.setupPasswordAcknowledged, isTrue);
      expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'existing Linux installation starts without generating a password',
    (tester) async {
      final c = await showSetup(tester);
      await press(tester, 'onboarding-start');
      expect(c.setupActions, ['start']);
      c.publish(ServiceSetupPhase.ready, view: 'login');
      await tester.pumpAndSettle();
      expect(
        find.textContaining('Use your existing Caspian password'),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('onboarding-password')), findsNothing);
      await press(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'Windows setup is direct and acknowledges the installer password',
    (tester) async {
      final c = await showSetup(tester, platform: TargetPlatform.windows);
      expect(
        find.textContaining('Windows will ask you to approve'),
        findsOneWidget,
      );
      await press(tester, 'onboarding-install');
      expect(c.setupActions, ['install']);
      c.publish(ServiceSetupPhase.ready, view: 'login');
      await tester.pumpAndSettle();
      expect(
        find.textContaining('Use the Caspian password you chose in Setup'),
        findsOneWidget,
      );
      expect(
        tester
            .widget<FilledButton>(
              find.byKey(const ValueKey('onboarding-continue')),
            )
            .onPressed,
        isNull,
      );
      await press(tester, 'onboarding-saved');
      await press(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'failed service check retains issued password and retries the same action',
    (tester) async {
      final c = await showSetup(tester);
      await press(tester, 'onboarding-start');
      c.publish(
        ServiceSetupPhase.failed,
        password: 'example-retained-password',
        problem: 'Service is still unavailable.',
      );
      await tester.pumpAndSettle();
      expect(find.text('Service is still unavailable.'), findsOneWidget);
      expect(find.text('example-retained-password'), findsOneWidget);
      expect(find.byKey(const ValueKey('onboarding-continue')), findsNothing);
      await press(tester, 'onboarding-retry');
      expect(c.setupActions, ['start', 'start']);
      c.publish(
        ServiceSetupPhase.ready,
        password: 'example-retained-password',
        view: 'setup',
      );
      await tester.pumpAndSettle();
      await press(tester, 'onboarding-saved');
      await press(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-password')), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'setup cancellation offers recovery and never claims readiness',
    (tester) async {
      final c = await showSetup(tester);
      await press(tester, 'onboarding-install');
      c.publish(
        ServiceSetupPhase.failed,
        problem: 'Administrator approval was declined.',
      );
      await tester.pumpAndSettle();
      expect(find.text('Administrator approval was declined.'), findsOneWidget);
      expect(
        find.textContaining('The background services are ready.'),
        findsNothing,
      );
      await press(tester, 'onboarding-recovery');
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('Start services'), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'web account setup never exposes desktop installation helpers',
    (tester) async {
      final c = await showSetup(tester, desktop: false);
      expect(find.byType(ServiceOnboarding), findsNothing);
      expect(find.byKey(const ValueKey('onboarding-install')), findsNothing);
      c.state = fixture(view: 'setup');
      await c.refresh();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-password')), findsOneWidget);
      expect(find.byKey(const ValueKey('setup-confirm')), findsOneWidget);
      expect(find.byType(ServiceOnboarding), findsNothing);
    },
  );

  for (final fa in [false, true]) {
    for (final platform in [
      TargetPlatform.macOS,
      TargetPlatform.windows,
      TargetPlatform.linux,
    ]) {
      onboardingTestWidgets(
        'narrow ${fa ? 'Persian' : 'English'} ${platform.name} setup stays usable',
        (tester) async {
          final c = await showSetup(
            tester,
            platform: platform,
            fa: fa,
            size: const Size(360, 800),
          );
          expect(tester.takeException(), isNull);
          expect(
            Directionality.of(tester.element(find.byType(ServiceOnboarding))),
            fa ? TextDirection.rtl : TextDirection.ltr,
          );
          await press(tester, 'onboarding-install');
          c.publish(ServiceSetupPhase.checking);
          await tester.pump();
          c.publish(ServiceSetupPhase.failed);
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull);
          await press(tester, 'onboarding-retry');
          c.publish(
            ServiceSetupPhase.ready,
            password: 'example-narrow-password',
            view: 'login',
          );
          await tester.pumpAndSettle();
          await press(tester, 'onboarding-saved');
          await press(tester, 'onboarding-continue');
          await tester.pumpAndSettle();
          expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
          expect(tester.takeException(), isNull);
        },
      );
    }
  }

  onboardingTestWidgets(
    'checklist requires saved settings healthy traffic and a joined device',
    (tester) async {
      final c = await showSetup(tester, desktop: false);
      c.state = fixture();
      final page = c.state['page'] as Map<String, dynamic>;
      page.addAll({
        'SetupIncomplete': true,
        'HasConfig': false,
        'HotspotReady': false,
        'Running': false,
        'Connected': false,
        'DeviceCount': 0,
      });
      await c.refresh();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('checklist-ready')), findsNothing);
      await press(tester, 'checklist-config');
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('config-config')).hitTestable(),
        findsOneWidget,
      );
      page.addAll({
        'SetupIncomplete': false,
        'HasConfig': true,
        'HotspotReady': true,
        'Running': true,
        'Connected': true,
      });
      await c.refresh();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-checklist')), findsOneWidget);
      expect(find.byKey(const ValueKey('checklist-ready')), findsNothing);
      page['DeviceCount'] = 1;
      page['TrafficCut'] = true;
      await c.refresh();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('checklist-ready')), findsNothing);
      page['TrafficCut'] = false;
      page['HasProblem'] = true;
      await c.refresh();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('checklist-ready')), findsNothing);
      page['HasProblem'] = false;
      await c.refresh();
      await tester.pumpAndSettle();
      await press(tester, 'checklist-ready');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-checklist')), findsNothing);
    },
  );

  onboardingTestWidgets(
    'ready service cannot bypass generated password acknowledgement',
    (tester) async {
      final c = await showSetup(tester);
      c.publish(
        ServiceSetupPhase.ready,
        password: 'example-private-password',
        view: 'dashboard',
      );
      await tester.pumpAndSettle();
      expect(find.byType(ServiceOnboarding), findsOneWidget);
      expect(find.byKey(const ValueKey('power')), findsNothing);
      await press(tester, 'onboarding-saved');
      await press(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('power')), findsOneWidget);
    },
  );

  onboardingTestWidgets(
    'readiness retry checks the service without reinstalling',
    (tester) async {
      final c = await showSetup(tester);
      c.readinessRetry = true;
      c.publish(
        ServiceSetupPhase.failed,
        password: 'example-saved-password',
        problem: 'Service is not responding yet.',
      );
      await tester.pumpAndSettle();
      expect(find.text('Check again'), findsOneWidget);
      await press(tester, 'onboarding-retry');
      expect(c.readinessChecks, 1);
      expect(c.setupActions, isEmpty);
      expect(c.setupPassword, 'example-saved-password');
      c.publish(ServiceSetupPhase.failed);
      await tester.pumpAndSettle();
    },
  );

  onboardingTestWidgets(
    'Windows without a Caspian password proceeds to account creation without false acknowledgement',
    (tester) async {
      final c = await showSetup(tester, platform: TargetPlatform.windows);
      c.publish(ServiceSetupPhase.ready, view: 'setup');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('onboarding-saved')), findsNothing);
      expect(
        find.textContaining('Choose your Caspian password'),
        findsOneWidget,
      );
      await press(tester, 'onboarding-continue');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-password')), findsOneWidget);
    },
  );

  for (final fa in [false, true]) {
    for (final platform in [
      TargetPlatform.macOS,
      TargetPlatform.windows,
      TargetPlatform.linux,
    ]) {
      onboardingTestWidgets(
        '${platform.name} ${fa ? 'Persian' : 'English'} install is visible at 800 by 600 without scrolling',
        (tester) async {
          await showSetup(
            tester,
            platform: platform,
            fa: fa,
            size: const Size(800, 600),
          );
          expect(
            find.byKey(const ValueKey('onboarding-install')).hitTestable(),
            findsOneWidget,
          );
        },
      );
    }
  }

  for (final platform in [
    TargetPlatform.macOS,
    TargetPlatform.windows,
    TargetPlatform.linux,
  ]) {
    onboardingTestWidgets(
      '${platform.name} ready first launch explains sign in once across restarts',
      (tester) async {
        final flags = <String, bool>{};
        OnboardingPreferences preferences() => OnboardingPreferences(
          read: (key) async => flags[key],
          write: (key, value) async {
            flags[key] = value;
          },
        );
        final c = await showSetup(
          tester,
          platform: platform,
          preferences: preferences(),
        );
        c.publish(ServiceSetupPhase.idle, view: 'login');
        await tester.pumpAndSettle();
        expect(find.byType(ServiceOnboarding), findsOneWidget);
        expect(find.byKey(const ValueKey('onboarding-install')), findsNothing);
        expect(c.setupActions, isEmpty);
        if (platform == TargetPlatform.windows) {
          await press(tester, 'onboarding-saved');
        }
        await press(tester, 'onboarding-continue');
        await tester.pumpAndSettle();
        expect(await preferences().welcomeCompleted(), isTrue);
        await tester.pumpWidget(
          CaspianApp(controller: c, onboardingPreferences: preferences()),
        );
        await tester.pumpAndSettle();
        expect(find.byType(ServiceOnboarding), findsNothing);
        expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
        expect(flags.values, everyElement(isTrue));
      },
    );
  }

  onboardingTestWidgets(
    'completed checklist persists and can be reopened without changing its saved flag',
    (tester) async {
      final flags = <String, bool>{};
      OnboardingPreferences preferences() => OnboardingPreferences(
        read: (key) async => flags[key],
        write: (key, value) async {
          flags[key] = value;
        },
      );
      final c = await showSetup(
        tester,
        desktop: false,
        preferences: preferences(),
      );
      c.state = fixture();
      (c.state['page'] as Map<String, dynamic>)['DeviceCount'] = 1;
      await c.refresh();
      await tester.pumpAndSettle();
      await press(tester, 'checklist-ready');
      await tester.pumpAndSettle();
      expect(await preferences().checklistCompleted(), isTrue);
      await press(tester, 'setup-guide');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-checklist')), findsOneWidget);
      expect(await preferences().checklistCompleted(), isTrue);
      await tester.pumpWidget(
        CaspianApp(controller: c, onboardingPreferences: preferences()),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('setup-checklist')), findsNothing);
    },
  );

  onboardingTestWidgets(
    'unavailable service still offers recovery after welcome was completed',
    (tester) async {
      await showSetup(
        tester,
        preferences: OnboardingPreferences(
          read: (_) async => true,
          write: (_, _) async {},
        ),
      );
      expect(find.byKey(const ValueKey('onboarding-start')), findsOneWidget);
      await press(tester, 'onboarding-recovery');
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
    },
  );

  for (final fa in [false, true]) {
    for (final narrow in [false, true]) {
      onboardingTestWidgets(
        'first-run ${fa ? 'Persian' : 'English'} ${narrow ? 'narrow' : 'desktop'} visual baseline',
        (tester) async {
          await showSetup(
            tester,
            platform: TargetPlatform.macOS,
            fa: fa,
            size: narrow ? const Size(360, 800) : const Size(800, 600),
          );
          expect(tester.takeException(), isNull);
          await expectLater(
            find.byKey(const ValueKey('first-run-screenshot')),
            matchesGoldenFile(
              'goldens/onboarding-${fa ? 'fa' : 'en'}-${narrow ? 'narrow' : 'desktop'}.png',
            ),
          );
        },
      );
    }
  }
}
