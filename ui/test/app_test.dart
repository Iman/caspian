// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:flutter/rendering.dart';
import 'package:caspian/app.dart';
import 'package:caspian/api_client.dart';
import 'package:caspian/controller.dart';
import 'package:caspian/onboarding_preferences.dart';
import 'package:caspian/transport.dart';
import 'package:caspian/theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

OnboardingPreferences completedOnboardingPreferences() =>
    OnboardingPreferences(read: (_) async => true, write: (_, _) async {});

class PollingTransport implements Transport {
  int requests = 0;
  final pending = Completer<TransportResponse>();

  @override
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form}) async {
    requests++;
    if (requests == 1) {
      return TransportResponse(200, jsonEncode(fixture()));
    }
    return pending.future;
  }

  @override
  void close() {}
}

class AuthValidationTransport implements Transport {
  AuthValidationTransport(this.view);
  final String view;
  int reads = 0;

  @override
  Future<TransportResponse> send(Uri uri, {Map<String, String>? form}) async {
    final reply = fixture(view: view);
    if (form == null) {
      reads++;
    } else {
      reply['ok'] = false;
      (reply['page'] as Map<String, dynamic>).addAll({
        'HasProblem': true,
        'ProblemHeadline': 'Check the password',
        'ProblemAdvice': 'Correct the submitted password and try again.',
      });
    }
    return TransportResponse(form == null ? 200 : 422, jsonEncode(reply));
  }

  @override
  void close() {}
}

class FakeController extends CaspianController {
  FakeController({bool desktop = false}) : super(desktop: desktop);
  Map<String, dynamic>? response;
  bool identifiersFail = false;
  @override
  Future<Map<String, dynamic>> identifiers() async {
    if (identifiersFail) throw StateError('unavailable');
    return {'uuid': 'example-generated-value', 'imei': 'example-test-value'};
  }

  @override
  Future<void> setLanguage(String value) async {
    language = value;
    (state['page'] as Map)['Lang'] = value;
    (state['page'] as Map)['Dir'] = value == 'fa' ? 'rtl' : 'ltr';
    notifyListeners();
  }

  final calls = <(String, Map<String, String>)>[];
  @override
  Future<void> act(String action, Map<String, String> values) async {
    calls.add((action, values));
    if (response != null) {
      state = response!;
      notifyListeners();
    }
  }

  @override
  Future<void> refresh() async {
    notifyListeners();
  }

  @override
  Future<void> setAdvanced(bool value) async {
    (state['page'] as Map)['Advanced'] = value;
    notifyListeners();
  }

  @override
  Future<void> serviceAction(String action) async {
    calls.add((action, {}));
  }
}

Map<String, dynamic> fixture({
  String view = 'dashboard',
  bool fa = false,
  bool cut = false,
}) => {
  'view': view,
  'ok': true,
  'page': <String, dynamic>{
    'Lang': fa ? 'fa' : 'en',
    'Dir': fa ? 'rtl' : 'ltr',
    'Running': true,
    'Connected': !cut,
    'TrafficCut': cut,
    'HeroClass': cut ? 'cut' : 'ok',
    'StatusWord': fa ? 'متصل' : 'Connected',
    'NextLabel': 'What to do now',
    'NextStep': 'Join this WiFi network.',
    'HotspotReady': true,
    'QR': File('test/fixtures/wifi.svg').readAsStringSync(),
    'SSID': 'Example WiFi',
    'Passphrase': 'example-password',
    'DeviceLine': 'No devices have joined yet',
    'HasConfig': true,
    'ConfigName': 'example-server',
    'ConfigSummary': 'vless 203.0.113.10',
    'Tiles': [
      {'Label': 'Connection', 'Value': 'Connected'},
      {'Label': 'Devices', 'Value': '0'},
      {'Label': 'Config', 'Value': 'example-server'},
      {'Label': 'Running for', 'Value': '1 minute'},
    ],
    'Events': [
      {'At': '12:00', 'Text': 'Signed in.'},
    ],
  },
  'strings': <String, dynamic>{
    'status.heading': 'Connection status',
    'wifi.heading': fa ? 'وای‌فای شما' : 'Your WiFi',
    'config.heading': 'Your config',
    'events.heading': 'What has happened',
    'connections.heading': 'Connections',
    'advanced.heading': 'Advanced',
    'help.heading': 'Help',
    'nav.signout': 'Sign out',
    'power.off': 'Switch off',
    'power.on': 'Switch on',
    'cut.switchlabel': 'Client traffic',
    'cut.caption': 'Keep WiFi available.',
    'power.caption': 'Switch the whole gateway off.',
    'wifi.name': 'Network name',
    'wifi.password': 'Password',
    'wifi.change': 'Change WiFi',
    'config.replace': 'Replace the config',
    'config.paste.label': 'Paste your config',
    'config.name.label': 'Config name',
    'config.submit': 'Save config',
    'password.heading': 'Change password',
    'password.current': 'Current password',
    'password.new': 'New password',
    'password.confirm': 'Confirm password',
    'password.save': 'Save password',
    'advanced.show': 'Show advanced settings',
    'advanced.hide': 'Hide advanced settings',
    'advanced.internet': 'Internet interface',
    'advanced.hotspotiface': 'Hotspot interface',
    'advanced.band': 'Band',
    'advanced.save': 'Save settings',
    'login.heading': 'Sign in',
    'login.password': 'Password',
    'login.submit': 'Sign in',
    'setup.heading': 'Set a password',
    'setup.password': 'Password',
    'setup.confirm': 'Confirm password',
    'setup.submit': 'Continue',
    'help.what.heading': 'What Caspian does',
    'nav.back': 'Back to connection status',
    'identifiers.heading': 'Test identifiers',
    'identifiers.generate': 'Generate',
  },
};

Map<String, String> catalog(bool fa) {
  final source = File('../internal/panel/i18n_messages.go').readAsStringSync();
  final block = source
      .split('var messages${fa ? 'FA' : 'EN'} = map[Key]string{')[1]
      .split('\n}')[0];
  final entries = RegExp(
    r'^\s*("[^"\n]+"):\s*("(?:[^"\\]|\\.)*")\s*,',
    multiLine: true,
  ).allMatches(block);
  return {
    for (final e in entries)
      jsonDecode(e.group(1)!) as String: jsonDecode(e.group(2)!) as String,
  };
}

Future<FakeController> showApp(
  WidgetTester tester, {
  Size size = const Size(1440, 1000),
  String view = 'dashboard',
  bool fa = false,
  bool cut = false,
  bool desktop = false,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final c = FakeController(desktop: desktop)
    ..state = fixture(view: view, fa: fa, cut: cut);
  addTearDown(c.dispose);
  await tester.pumpWidget(
    RepaintBoundary(
      key: const ValueKey('screenshot'),
      child: CaspianApp(
        controller: c,
        onboardingPreferences: completedOnboardingPreferences(),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return c;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() async {
    final loader = FontLoader('Vazirmatn')
      ..addFont(rootBundle.load('assets/fonts/Vazirmatn.ttf'));
    await loader.load();
    final icons = FontLoader('MaterialIcons')
      ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
    await icons.load();
    final mono = FontLoader('CaspianMono')
      ..addFont(rootBundle.load('assets/fonts/NotoSansMono.ttf'));
    await mono.load();
  });
  testWidgets('dashboard retains rail and exposes help and logout', (
    tester,
  ) async {
    await showApp(tester);
    expect(find.text('Caspian'), findsOneWidget);
    expect(find.text('Help'), findsOneWidget);
    expect(find.text('Sign out'), findsOneWidget);
    expect(CaspianStyle.teal, const Color(0xFF097C87));
    expect(tester.takeException(), isNull);
  });

  testWidgets('traffic cut never changes the power action to start', (
    tester,
  ) async {
    final c = await showApp(tester, cut: true);
    expect(find.text('Switch off'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('power')));
    expect(c.calls.single.$1, 'power');
    expect(c.calls.single.$2, {'on': '0'});
    await tester.tap(find.byKey(const ValueKey('traffic')));
    expect(c.calls.last.$1, 'cut');
    expect(c.calls.last.$2, {'cut': '0'});
  });

  testWidgets('login submits password from the native field', (tester) async {
    final c = await showApp(tester, view: 'login');
    await tester.enterText(
      find.byKey(const ValueKey('login-password')),
      'example-password',
    );
    await tester.tap(find.byKey(const ValueKey('login-submit')));
    expect(c.calls.single.$1, 'login');
    expect(c.calls.single.$2, {'password': 'example-password'});
    expect(find.byType(Drawer), findsNothing);
  });

  testWidgets('setup includes confirmation', (tester) async {
    final c = await showApp(tester, view: 'setup');
    await tester.enterText(
      find.byKey(const ValueKey('setup-password')),
      'example-password',
    );
    await tester.enterText(
      find.byKey(const ValueKey('setup-confirm')),
      'example-password',
    );
    await tester.tap(find.byKey(const ValueKey('setup-submit')));
    expect(c.calls.single.$1, 'setup');
    expect(c.calls.single.$2, {
      'password': 'example-password',
      'confirm': 'example-password',
    });
  });

  for (final fa in [false, true]) {
    testWidgets('narrow ${fa ? 'Persian' : 'English'} screen has no overflow', (
      tester,
    ) async {
      await showApp(tester, size: const Size(360, 800), fa: fa);
      expect(tester.takeException(), isNull);
      final context = tester.element(find.byKey(const ValueKey('power')));
      expect(
        Directionality.of(context),
        fa ? TextDirection.rtl : TextDirection.ltr,
      );
      final credential = tester.widget<SelectableText>(
        find.byWidgetPredicate(
          (w) => w is SelectableText && w.data == 'example-password',
        ),
      );
      expect(credential.textDirection, TextDirection.ltr);
      await tester.drag(
        find.byType(SingleChildScrollView).first,
        const Offset(0, -1500),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('server refresh preserves an unfinished config draft', (
    tester,
  ) async {
    final c = await showApp(tester);
    await tester.ensureVisible(find.text('Replace the config'));
    await tester.tap(find.text('Replace the config'));
    await tester.pumpAndSettle();
    final field = find.byKey(const ValueKey('config-config'));
    await tester.ensureVisible(field);
    await tester.enterText(field, 'example draft');
    await c.refresh();
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(field).controller!.text, 'example draft');
  });

  testWidgets('help stays inside the application', (tester) async {
    await showApp(tester);
    await tester.tap(find.text('Help'));
    await tester.pumpAndSettle();
    expect(find.text('What Caspian does'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('desktop recovery remains available without service state', (
    tester,
  ) async {
    final c = await showApp(tester, desktop: true);
    c.state = {};
    await c.refresh();
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Service recovery'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Start services'));
    expect(c.calls.single.$1, 'start');
  });

  testWidgets('login input has an accessible name', (tester) async {
    await showApp(tester, view: 'login');
    final handle = tester.ensureSemantics();
    await tester.pump();
    expect(
      tester
          .getSemantics(
            find.byWidgetPredicate(
              (w) =>
                  w is Semantics && w.properties.identifier == 'login-password',
            ),
          )
          .label,
      'Password',
    );
    handle.dispose();
  });

  testWidgets('rejected login displays server advice and preserves password', (
    tester,
  ) async {
    final c = await showApp(tester, view: 'login');
    c.response = fixture(view: 'login')..['ok'] = false;
    (c.response!['page'] as Map<String, dynamic>).addAll({
      'HasProblem': true,
      'ProblemHeadline': 'Password rejected',
      'ProblemAdvice': 'Try again.',
    });
    await tester.enterText(
      find.byKey(const ValueKey('login-password')),
      'example-wrong-password',
    );
    await tester.tap(find.byKey(const ValueKey('login-submit')));
    await tester.pumpAndSettle();
    expect(find.text('Password rejected\nTry again.'), findsOneWidget);
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('login-password')))
          .controller!
          .text,
      'example-wrong-password',
    );
  });

  testWidgets('advanced settings submit independently of recovery', (
    tester,
  ) async {
    final c = await showApp(tester);
    (c.state['page'] as Map<String, dynamic>).addAll({
      'Advanced': true,
      'PanelOnLAN': true,
      'ChannelPinned': true,
      'Interfaces': [
        {'Name': 'eth-test', 'Kind': 'Ethernet', 'CanHost': false},
        {'Name': 'wifi-test', 'Kind': 'WiFi', 'CanHost': true},
      ],
      'Bands': [
        {'Value': '2.4', 'Words': '2.4 GHz', 'Selected': true},
      ],
      'Channels': [
        {'Value': '6', 'Selected': true},
      ],
      'CurrentInternet': 'eth-test',
      'CurrentHotspot': 'wifi-test',
      'CurrentBand': '2.4',
      'CurrentChannel': '6',
      'CurrentCountry': 'GB',
      'CurrentSubnet': '192.168.50.0/24',
      'LogLevels': [
        {'Value': 'warning', 'Selected': true},
      ],
      'FixedFacts': [
        {'Label': 'Tunnel policy', 'Words': 'Block'},
      ],
      'ConfigFacts': [
        {'Label': 'Transport', 'Value': 'tcp'},
      ],
      'EnginePhase': 'running',
      'EngineReason': 'example reason',
      'EngineLog': [
        {'At': '12:01', 'Text': 'example engine log'},
      ],
    });
    await c.refresh();
    await tester.pumpAndSettle();
    final country = find.byKey(const ValueKey('advanced-country'));
    await tester.ensureVisible(country);
    await tester.enterText(country, 'DE');
    final submit = find.byKey(const ValueKey('advanced-submit')).last;
    await tester.ensureVisible(submit);
    await tester.tap(submit);
    expect(c.calls.last.$2['country'], 'DE');
    expect(c.calls.last.$2['panel_on_lan'], '1');
    expect(c.calls.last.$2['channel'], '6');
    expect(c.calls.last.$2.containsKey('connections_only'), isFalse);
    expect(tester.takeException(), isNull);
  });

  testWidgets('new hotspot and config work without existing settings', (
    tester,
  ) async {
    final c = await showApp(tester);
    (c.state['page'] as Map<String, dynamic>).addAll({
      'HotspotReady': false,
      'HasConfig': false,
      'SetupIncomplete': true,
      'Running': false,
      'HeroClass': 'off',
      'SSID': '',
      'Passphrase': '',
      'SuggestedSSID': 'Example WiFi',
      'SuggestedPassphrase': 'example-password',
      'Events': [],
    });
    await c.refresh();
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('traffic')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('power')));
    expect(c.calls.last.$2, {'on': '1'});
    final hotspot = find.byKey(const ValueKey('hotspot-submit'));
    await tester.ensureVisible(hotspot);
    await tester.tap(hotspot);
    expect(c.calls.last.$2, {
      'ssid': 'Example WiFi',
      'passphrase': 'example-password',
    });
    final config = find.byKey(const ValueKey('config-config'));
    await tester.ensureVisible(config);
    await tester.enterText(config, 'example-config');
    await tester.ensureVisible(find.byKey(const ValueKey('config-submit')));
    await tester.tap(find.byKey(const ValueKey('config-submit')));
    expect(c.calls.last.$2['config'], 'example-config');
    expect(tester.widget<TextField>(config).controller!.text, isEmpty);
  });

  testWidgets(
    'identifier generation error is visible without losing dashboard',
    (tester) async {
      final c = await showApp(tester);
      (c.state['strings'] as Map)['identifiers.failed'] = 'Generation failed';
      c.identifiersFail = true;
      await c.refresh();
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Generate'));
      await tester.tap(find.text('Generate'));
      await tester.pumpAndSettle();
      expect(find.text('Generation failed'), findsOneWidget);
      expect(c.state['view'], 'dashboard');
    },
  );

  testWidgets(
    'generated identifiers are shown only for the signed in session',
    (tester) async {
      final c = await showApp(tester);
      await tester.ensureVisible(find.text('Generate'));
      await tester.tap(find.text('Generate'));
      await tester.pumpAndSettle();
      expect(find.text('example-generated-value'), findsOneWidget);
      c.state = fixture(view: 'login');
      await c.refresh();
      await tester.pumpAndSettle();
      expect(find.text('example-generated-value'), findsNothing);
      c.state = fixture();
      await c.refresh();
      await tester.pumpAndSettle();
      expect(find.text('example-generated-value'), findsNothing);
    },
  );

  testWidgets('busy controller disables power and sign in submission', (
    tester,
  ) async {
    final c = await showApp(tester);
    c.busy = true;
    await c.refresh();
    await tester.pump();
    expect(
      tester
          .widget<OutlinedButton>(find.byKey(const ValueKey('power')))
          .onPressed,
      isNull,
    );
    c.state = fixture(view: 'login');
    await c.refresh();
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('login-submit')))
          .onPressed,
      isNull,
    );
    c.busy = false;
  });

  testWidgets('language selection mirrors the rail', (tester) async {
    final c = await showApp(tester);
    await tester.tap(find.byTooltip('Language'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('فارسی').last);
    await tester.pumpAndSettle();
    expect(c.language, 'fa');
    expect(
      Directionality.of(tester.element(find.byKey(const ValueKey('power')))),
      TextDirection.rtl,
    );
  });

  for (final fa in [false, true]) {
    testWidgets('dashboard ${fa ? 'Persian' : 'English'} visual baseline', (
      tester,
    ) async {
      final c = await showApp(tester, fa: fa, size: const Size(1624, 1100));
      c.state['strings'] = catalog(fa);
      final words = c.state['strings'] as Map<String, String>;
      final page = c.state['page'] as Map<String, dynamic>;
      page['NextLabel'] = words['next.label'];
      page['NextStep'] = words['next.join'];
      page['DeviceLine'] = words['devices.none'];
      page['Tiles'] = [
        {'Label': words['tile.status'], 'Value': words['status.connected']},
        {'Label': words['tile.devices'], 'Value': '0'},
        {'Label': words['tile.config'], 'Value': 'example-server'},
        {'Label': words['tile.uptime'], 'Value': fa ? '1 دقیقه' : '1 minute'},
      ];
      page['Events'] = [];
      await c.refresh();
      await tester.pumpAndSettle();
      await expectLater(
        find.byKey(const ValueKey('screenshot')),
        matchesGoldenFile('goldens/dashboard-${fa ? 'fa' : 'en'}.png'),
      );
    });
  }
  testWidgets('WiFi join QR paints a white quiet zone without CSS', (
    tester,
  ) async {
    await showApp(tester);
    final qr = find.byKey(const ValueKey('wifi-qr'));
    await tester.ensureVisible(qr);
    await tester.pumpAndSettle();
    final boundary = tester.renderObject<RenderRepaintBoundary>(qr);
    final bytes = await tester.runAsync(() async {
      final image = await boundary.toImage();
      final pixels = await image.toByteData(format: ui.ImageByteFormat.rawRgba);
      final sample = pixels!.buffer
          .asUint8List()
          .skip((10 * image.width + 10) * 4)
          .take(4)
          .toList();
      image.dispose();
      return sample;
    });
    expect(bytes, [255, 255, 255, 255]);
  });

  testWidgets('service failure retains the selected Persian language', (
    tester,
  ) async {
    final c = await showApp(tester, fa: true, desktop: true);
    c.language = 'fa';
    c.state = {};
    await c.refresh();
    await tester.pumpAndSettle();
    expect(find.text('راه‌اندازی کاسپین'), findsOneWidget);
    expect(
      Directionality.of(tester.element(find.byType(Scaffold))),
      TextDirection.rtl,
    );
  });
  testWidgets('renderer default fallback font is bundled locally', (
    tester,
  ) async {
    final manifest =
        jsonDecode(await rootBundle.loadString('FontManifest.json')) as List;
    final fallback = manifest.where((family) => family['family'] == 'Roboto');
    expect(
      fallback,
      hasLength(1),
      reason:
          'CanvasKit otherwise requests its default Roboto font from the fallback URL',
    );
    final asset = fallback.single['fonts'].single['asset'] as String;
    expect(Uri.parse(asset).hasScheme, isFalse);
    expect((await rootBundle.load(asset)).lengthInBytes, greaterThan(0));
  });
  testWidgets('Persian technical values retain the bundled Persian fallback', (
    tester,
  ) async {
    final c = await showApp(tester, fa: true);
    (c.state['page'] as Map<String, dynamic>).addAll({
      'SSID': 'شبکهٔ نمونه',
      'ConfigSummary': 'پیکربندی نمونه',
    });
    await c.refresh();
    await tester.pumpAndSettle();
    for (final value in ['شبکهٔ نمونه', 'پیکربندی نمونه']) {
      final editable = tester.widget<EditableText>(
        find.descendant(
          of: find.widgetWithText(SelectableText, value),
          matching: find.byType(EditableText),
        ),
      );
      expect(editable.style.fontFamily, 'CaspianMono');
      expect(
        editable.style.fontFamilyFallback,
        contains('Vazirmatn'),
        reason:
            'The bundled mono and Latin fonts do not contain Persian glyphs',
      );
    }
  });
  testWidgets(
    'setup transition exposes sidebar control identifiers and actions',
    (tester) async {
      final handle = tester.ensureSemantics();
      try {
        final c = await showApp(tester, view: 'setup');
        c.response = fixture();
        await tester.enterText(
          find.byKey(const ValueKey('setup-password')),
          'example-password',
        );
        await tester.enterText(
          find.byKey(const ValueKey('setup-confirm')),
          'example-password',
        );
        await tester.tap(find.byKey(const ValueKey('setup-submit')));
        await tester.pumpAndSettle();
        for (final id in [
          'logout',
          'language',
          'nav-status.heading',
          'nav-wifi.heading',
          'nav-config.heading',
        ]) {
          final control = find.byWidgetPredicate(
            (w) => w is Semantics && w.properties.identifier == id,
          );
          expect(control, findsOneWidget);
          final data = tester.getSemantics(control).getSemanticsData();
          expect(data.identifier, id);
          expect(data.hasAction(ui.SemanticsAction.tap), isTrue, reason: id);
        }
      } finally {
        handle.dispose();
      }
    },
  );

  testWidgets(
    'periodic refresh preserves focused password and unfinished input',
    (tester) async {
      final transport = PollingTransport();
      final c = CaspianController(
        client: ApiClient(transport: transport),
        desktop: false,
      );
      await tester.pumpWidget(
        CaspianApp(
          controller: c,
          onboardingPreferences: completedOnboardingPreferences(),
        ),
      );
      c.start();
      await tester.pumpAndSettle();
      final field = find.byKey(const ValueKey('password-confirm'));
      await tester.ensureVisible(field);
      await tester.enterText(field, 'example-unfinished');
      final editable = tester.widget<EditableText>(
        find.descendant(of: field, matching: find.byType(EditableText)),
      );
      expect(editable.focusNode.hasFocus, isTrue);
      await tester.pump(const Duration(seconds: 5));
      await tester.pump();
      expect(transport.requests, 2);
      final focusedDuringPoll = editable.focusNode.hasFocus;
      final valueDuringPoll = editable.controller.text;
      transport.pending.complete(TransportResponse(200, jsonEncode(fixture())));
      await tester.pumpAndSettle();
      final focusedAfterPoll = editable.focusNode.hasFocus;
      await tester.pumpWidget(const SizedBox());
      c.dispose();
      expect(valueDuringPoll, 'example-unfinished');
      expect(
        focusedDuringPoll,
        isTrue,
        reason: 'Reading status must not interrupt typing',
      );
      expect(focusedAfterPoll, isTrue);
    },
  );

  testWidgets('Enter during status polling does not clear an unsent password', (
    tester,
  ) async {
    final transport = PollingTransport();
    final c = CaspianController(
      client: ApiClient(transport: transport),
      desktop: false,
    );
    await tester.pumpWidget(
      CaspianApp(
        controller: c,
        onboardingPreferences: completedOnboardingPreferences(),
      ),
    );
    c.start();
    await tester.pumpAndSettle();
    final field = find.byKey(const ValueKey('password-confirm'));
    await tester.ensureVisible(field);
    await tester.enterText(field, 'example-unfinished');
    await tester.pump(const Duration(seconds: 5));
    await tester.pump();
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pump();
    final valueDuringPoll = tester.widget<TextField>(field).controller!.text;
    final requestsDuringPoll = transport.requests;
    transport.pending.complete(TransportResponse(200, jsonEncode(fixture())));
    await tester.pumpAndSettle();
    await tester.pumpWidget(const SizedBox());
    c.dispose();
    expect(
      requestsDuringPoll,
      2,
      reason: 'The pending read serializes submission',
    );
    expect(
      valueDuringPoll,
      'example-unfinished',
      reason: 'No password change request was sent',
    );
  });

  for (final view in ['login', 'setup']) {
    testWidgets('$view validation remains visible past the polling interval', (
      tester,
    ) async {
      final transport = AuthValidationTransport(view);
      final c = CaspianController(
        client: ApiClient(transport: transport),
        desktop: false,
      );
      await tester.pumpWidget(
        CaspianApp(
          controller: c,
          onboardingPreferences: completedOnboardingPreferences(),
        ),
      );
      c.start();
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(ValueKey('$view-password')),
        'example-wrong-password',
      );
      if (view == 'setup') {
        await tester.enterText(
          find.byKey(const ValueKey('setup-confirm')),
          'example-mismatch',
        );
      }
      await tester.tap(find.byKey(ValueKey('$view-submit')));
      await tester.pumpAndSettle();
      expect(find.textContaining('Check the password'), findsOneWidget);
      await tester.pump(const Duration(seconds: 6));
      await tester.pumpAndSettle();
      final stillVisible = find
          .textContaining('Check the password')
          .evaluate()
          .length;
      final readsAfterTimer = transport.reads;
      await c.refresh();
      await tester.pumpAndSettle();
      final visibleAfterManualRefresh = find
          .textContaining('Check the password')
          .evaluate()
          .length;
      final readsAfterManualRefresh = transport.reads;
      await tester.pumpWidget(const SizedBox());
      c.dispose();
      expect(
        stillVisible,
        1,
        reason: 'Automatic polling must not erase explicit validation advice',
      );
      expect(readsAfterTimer, 1);
      expect(visibleAfterManualRefresh, 0);
      expect(
        readsAfterManualRefresh,
        2,
        reason: 'Manual refresh remains available',
      );
    });
  }

  testWidgets('sidebar controls keep their button roles during refresh', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    try {
      final c = await showApp(tester);
      c.busy = true;
      await c.refresh();
      await tester.pump();
      for (final id in ['logout', 'language', 'nav-status.heading']) {
        final control = find.byWidgetPredicate(
          (w) => w is Semantics && w.properties.identifier == id,
        );
        final data = tester.getSemantics(control).getSemanticsData();
        expect(data.flagsCollection.isButton, isTrue, reason: id);
        expect(data.hasAction(ui.SemanticsAction.tap), isFalse, reason: id);
      }
      c.busy = false;
    } finally {
      handle.dispose();
    }
  });
}
