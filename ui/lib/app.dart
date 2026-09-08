// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_svg/flutter_svg.dart';

import 'controller.dart';
import 'onboarding.dart';
import 'onboarding_preferences.dart';
import 'theme.dart';

Map<String, dynamic> _map(dynamic value) =>
    value is Map ? Map<String, dynamic>.from(value) : {};
List<Map<String, dynamic>> _rows(dynamic value) =>
    value is List ? value.map(_map).toList() : [];
String _str(dynamic value) => value?.toString() ?? '';

class CaspianApp extends StatelessWidget {
  const CaspianApp({
    super.key,
    required this.controller,
    this.onboardingPreferences,
  });
  final CaspianController controller;
  final OnboardingPreferences? onboardingPreferences;

  @override
  Widget build(BuildContext context) => AnimatedBuilder(
    animation: controller,
    builder: (context, _) {
      final page = _map(controller.state['page']);
      final lang = _str(page['Lang'] ?? controller.language);
      return MaterialApp(
        title: 'Caspian',
        debugShowCheckedModeBanner: false,
        theme: CaspianStyle.theme,
        supportedLocales: const [Locale('en'), Locale('fa')],
        localizationsDelegates: GlobalMaterialLocalizations.delegates,
        locale: Locale(lang.isEmpty ? 'en' : lang),
        home: Directionality(
          textDirection:
              (page['Dir'] ?? (lang == 'fa' ? 'rtl' : 'ltr')) == 'rtl'
              ? TextDirection.rtl
              : TextDirection.ltr,
          child: _Home(
            controller: controller,
            preferences: onboardingPreferences,
          ),
        ),
      );
    },
  );
}

class _Home extends StatefulWidget {
  const _Home({required this.controller, this.preferences});
  final CaspianController controller;
  final OnboardingPreferences? preferences;
  @override
  State<_Home> createState() => _HomeState();
}

class _HomeState extends State<_Home> {
  final _sections = {
    for (final name in [
      'setup',
      'status',
      'wifi',
      'config',
      'events',
      'connections',
      'advanced',
      'password',
      'identifiers',
    ])
      name: GlobalKey(),
  };
  final _scaffold = GlobalKey<ScaffoldState>();
  final _scroll = ScrollController();
  bool _help = false;
  bool _setupDismissed = false;
  bool _showSetupChecklist = false;
  bool _setupChecklistFinished = false;
  bool _preferencesLoaded = false;
  bool _welcomeCompleted = false;
  bool _checklistCompleted = false;
  late final OnboardingPreferences _preferences;
  Map<String, dynamic> _identifiers = {};
  CaspianController get c => widget.controller;
  Map<String, dynamic> get p => _map(c.state['page']);
  bool get fa => (p['Lang'] ?? c.language) == 'fa';
  String local(String en, String persian) => fa ? persian : en;
  String t(String key) {
    if (c.desktop) {
      switch (key) {
        case 'login.recovery.help':
        case 'help.trouble.forgot.a':
          return local(
            'Open Service recovery and choose Reset password. The operating system will request administrator access.',
            'بازیابی سرویس را باز کنید و بازنشانی گذرواژه را انتخاب کنید. سیستم‌عامل دسترسی مدیر را درخواست می‌کند.',
          );
        case 'help.trouble.lostpanel.q':
          return local(
            'Caspian cannot reach its service',
            'کاسپین به سرویس خود دسترسی ندارد',
          );
        case 'help.trouble.lostpanel.a':
          return local(
            'Open Service recovery and choose Start services, then refresh. If the services are not installed, run the Caspian installer.',
            'بازیابی سرویس را باز کنید، راه‌اندازی سرویس‌ها را بزنید و سپس تازه‌سازی کنید. اگر سرویس‌ها نصب نیستند، نصب‌کننده کاسپین را اجرا کنید.',
          );
        case 'help.two.lockout':
          return local(
            'Switching off disconnects devices from the hotspot. This window stays open so you can switch Caspian on again.',
            'خاموش کردن، دستگاه‌ها را از هات‌اسپات جدا می‌کند. این پنجره باز می‌ماند تا بتوانید کاسپین را دوباره روشن کنید.',
          );
      }
    }
    return _str(_map(c.state['strings'])[key]);
  }

  String value(String key) => _str(p[key]);
  bool flag(String key) => p[key] == true;

  @override
  void initState() {
    super.initState();
    _preferences = widget.preferences ?? OnboardingPreferences();
    _loadPreferences();
  }

  Future<void> _loadPreferences() async {
    final values = await Future.wait([
      _preferences.welcomeCompleted(),
      _preferences.checklistCompleted(),
    ]);
    if (!mounted) return;
    setState(() {
      _welcomeCompleted = values[0];
      _checklistCompleted = values[1];
      _preferencesLoaded = true;
    });
  }

  Future<void> _continueSetup() async {
    final saved = await _preferences.completeWelcome();
    if (!mounted) return;
    setState(() {
      _welcomeCompleted = saved;
      _setupDismissed = true;
    });
  }

  Future<void> _finishChecklist() async {
    final saved = await _preferences.completeChecklist();
    if (!mounted) return;
    setState(() {
      _checklistCompleted = saved;
      _showSetupChecklist = false;
      _setupChecklistFinished = true;
    });
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _jump(String name) {
    _scaffold.currentState?.closeDrawer();
    setState(() => _help = false);
    if (name == 'advanced' && !flag('Advanced')) {
      c.setAdvanced(true).then((_) => _reveal(name));
    } else {
      _reveal(name);
    }
  }

  void _reveal(String name) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final target = _sections[name]?.currentContext;
      if (target != null) {
        Scrollable.ensureVisible(
          target,
          duration: const Duration(milliseconds: 250),
        );
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    if (c.setupPhase == ServiceSetupPhase.installing ||
        c.setupPhase == ServiceSetupPhase.checking) {
      _setupDismissed = false;
    }
    final serviceSetupVisible =
        c.desktop && c.setupPhase != ServiceSetupPhase.idle && !_setupDismissed;
    final signedIn = c.state['view'] == 'dashboard' && !serviceSetupVisible;
    if (!signedIn) {
      _identifiers = {};
      _showSetupChecklist = false;
      _setupChecklistFinished = false;
    } else if (_preferencesLoaded &&
        (flag('SetupIncomplete') || !_checklistCompleted) &&
        !_setupChecklistFinished) {
      _showSetupChecklist = true;
    }
    return LayoutBuilder(
      builder: (context, constraints) {
        final wide = constraints.maxWidth >= 1000;
        return Scaffold(
          key: _scaffold,
          drawer: signedIn && !wide
              ? Drawer(backgroundColor: CaspianStyle.teal, child: _rail())
              : null,
          body: SafeArea(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (signedIn && wide)
                  SizedBox(width: CaspianStyle.railWidth, child: _rail()),
                Expanded(
                  child: Column(
                    children: [
                      Container(
                        decoration: const BoxDecoration(
                          color: CaspianStyle.surface,
                          border: Border(
                            bottom: BorderSide(color: CaspianStyle.edge),
                          ),
                        ),
                        padding: const EdgeInsets.symmetric(
                          horizontal: 24,
                          vertical: 12,
                        ),
                        child: Row(
                          children: [
                            if (signedIn && !wide)
                              IconButton(
                                tooltip: local('Menu', 'فهرست'),
                                icon: const Icon(Icons.menu),
                                onPressed: () =>
                                    _scaffold.currentState?.openDrawer(),
                              ),
                            Expanded(
                              child: Text(
                                signedIn
                                    ? t(
                                        _help
                                            ? 'help.heading'
                                            : 'status.heading',
                                      )
                                    : 'Caspian',
                                style: Theme.of(context).textTheme.titleMedium,
                              ),
                            ),
                            if (!signedIn) _languagePicker(),
                            if (signedIn)
                              IconButton(
                                key: const ValueKey('setup-guide'),
                                tooltip: local(
                                  'Setup guide',
                                  'راهنمای راه‌اندازی',
                                ),
                                icon: const Icon(Icons.checklist),
                                onPressed: () {
                                  setState(() {
                                    _help = false;
                                    _showSetupChecklist = true;
                                    _setupChecklistFinished = false;
                                  });
                                  _reveal('setup');
                                },
                              ),
                            IconButton(
                              tooltip: local('Refresh', 'تازه‌سازی'),
                              onPressed: c.busy ? null : c.refresh,
                              icon: const Icon(Icons.refresh),
                            ),
                            if (c.desktop)
                              IconButton(
                                tooltip: local(
                                  'Service recovery',
                                  'بازیابی سرویس',
                                ),
                                onPressed: _serviceDialog,
                                icon: const Icon(Icons.build_outlined),
                              ),
                          ],
                        ),
                      ),
                      if (c.busy)
                        const LinearProgressIndicator(
                          minHeight: 3,
                          color: CaspianStyle.teal,
                          backgroundColor: CaspianStyle.ground,
                        ),
                      Expanded(
                        child: SingleChildScrollView(
                          controller: _scroll,
                          padding: EdgeInsets.all(
                            constraints.maxWidth < 500 ? 16 : 24,
                          ),
                          child: Align(
                            alignment: Alignment.topCenter,
                            child: ConstrainedBox(
                              constraints: BoxConstraints(
                                maxWidth: signedIn
                                    ? CaspianStyle.contentMax
                                    : 460,
                              ),
                              child: _stack([
                                if (c.error != null)
                                  _banner(c.error!, CaspianStyle.coral),
                                if (signedIn)
                                  ...(_help ? _helpCards() : _dashboard())
                                else
                                  _auth(),
                                const Divider(height: 32),
                                Text(
                                  'Caspian ${value('Version')}',
                                  style: const TextStyle(
                                    color: CaspianStyle.quiet,
                                    fontSize: 14,
                                  ),
                                ),
                              ]),
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _languagePicker({bool rail = false}) {
    final languages = _rows(c.state['languages']);
    return MergeSemantics(
      child: Semantics(
        container: true,
        button: true,
        enabled: !c.busy,
        identifier: 'language',
        child: PopupMenuButton<String>(
          tooltip: local('Language', 'زبان'),
          enabled: !c.busy,
          onSelected: c.setLanguage,
          itemBuilder: (_) =>
              (languages.isEmpty
                      ? [
                          {'code': 'en', 'name': 'English'},
                          {'code': 'fa', 'name': 'فارسی'},
                        ]
                      : languages)
                  .map(
                    (l) => PopupMenuItem(
                      value: _str(l['code']),
                      child: Text(_str(l['name'])),
                    ),
                  )
                  .toList(),
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.language,
                  color: rail ? CaspianStyle.sage : CaspianStyle.teal,
                ),
                const SizedBox(width: 12),
                Text(
                  fa
                      ? 'فارسی'
                      : (languages
                                .where((l) => l['code'] == p['Lang'])
                                .firstOrNull?['name'] ??
                            'English'),
                  style: TextStyle(
                    color: rail ? CaspianStyle.surface : CaspianStyle.ink,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _rail() => Container(
    color: CaspianStyle.teal,
    child: Column(
      children: [
        const Padding(
          padding: EdgeInsets.fromLTRB(28, 24, 24, 24),
          child: Row(
            children: [
              Icon(Icons.shield_outlined, color: CaspianStyle.sage, size: 34),
              SizedBox(width: 12),
              Text(
                'Caspian',
                style: TextStyle(
                  color: CaspianStyle.surface,
                  fontSize: 22,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            children: [
              _nav(
                'status.heading',
                Icons.power_settings_new,
                () => _jump('status'),
              ),
              _nav('wifi.heading', Icons.wifi, () => _jump('wifi')),
              _nav('config.heading', Icons.key_outlined, () => _jump('config')),
              _nav(
                'events.heading',
                Icons.format_list_bulleted,
                () => _jump('events'),
              ),
              _nav(
                'connections.heading',
                Icons.lan_outlined,
                () => _jump('connections'),
              ),
              _nav('advanced.heading', Icons.tune, () => _jump('advanced')),
              _nav('help.heading', Icons.help_outline, () {
                _scaffold.currentState?.closeDrawer();
                setState(() => _help = true);
                _scroll.jumpTo(0);
              }),
            ],
          ),
        ),
        const Padding(
          padding: EdgeInsets.symmetric(horizontal: 16),
          child: Divider(color: CaspianStyle.ink),
        ),
        _languagePicker(rail: true),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 0, 16, 20),
          child: _nav('nav.signout', Icons.logout, () => c.act('logout', {})),
        ),
      ],
    ),
  );

  Widget _nav(String key, IconData icon, VoidCallback action) => MergeSemantics(
    child: Semantics(
      container: true,
      button: true,
      enabled: !c.busy,
      identifier: key == 'nav.signout' ? 'logout' : 'nav-$key',
      child: ListTile(
        enabled: !c.busy,
        contentPadding: const EdgeInsets.symmetric(horizontal: 12),
        leading: Icon(icon, color: CaspianStyle.sage, size: 22),
        title: Text(
          t(key),
          style: const TextStyle(
            color: CaspianStyle.ground,
            fontSize: 16,
            fontWeight: FontWeight.w600,
          ),
        ),
        onTap: c.busy ? null : action,
      ),
    ),
  );

  Widget _auth() {
    final setup = c.state['view'] == 'setup';
    if (c.desktop && c.state.isNotEmpty && !_preferencesLoaded) {
      return _card(
        local('Set up Caspian', 'راه‌اندازی کاسپین'),
        Icons.shield_outlined,
        [const LinearProgressIndicator()],
      );
    }
    if (c.desktop &&
        (c.state.isEmpty ||
            (!_welcomeCompleted &&
                !_setupDismissed &&
                {'login', 'setup'}.contains(c.state['view'])) ||
            (c.setupPhase != ServiceSetupPhase.idle && !_setupDismissed))) {
      return ServiceOnboarding(
        controller: c,
        onContinue: _continueSetup,
        onRecovery: _serviceDialog,
      );
    }
    if (c.state.isEmpty) {
      return _card(
        local('Connect to Caspian', 'اتصال به کاسپین'),
        Icons.shield_outlined,
        [
          Text(
            local(
              'Waiting for the Caspian service.',
              'در انتظار سرویس کاسپین.',
            ),
          ),
          OutlinedButton(
            onPressed: c.busy ? null : c.refresh,
            child: Text(local('Try again', 'تلاش دوباره')),
          ),
          if (c.desktop)
            OutlinedButton(
              onPressed: _serviceDialog,
              child: Text(local('Service recovery', 'بازیابی سرویس')),
            ),
        ],
      );
    }
    final prefix = setup ? 'setup' : 'login';
    return _card(t('$prefix.heading'), Icons.lock_outline, [
      ..._messages(),
      if (setup) Text(t('setup.intro')),
      _ActionForm(
        key: ValueKey(prefix),
        controller: c,
        action: prefix,
        submit: t('$prefix.submit'),
        fields: [
          _Field('password', t('$prefix.password'), secret: true),
          if (setup) _Field('confirm', t('setup.confirm'), secret: true),
        ],
      ),
      Text(t('$prefix.hint')),
      if (setup) Text(t('setup.writeitdown')),
      if (!setup)
        ExpansionTile(
          title: Text(t('login.recovery')),
          children: [
            Text(t('login.recovery.help')),
            if (c.desktop)
              OutlinedButton(
                onPressed: _serviceDialog,
                child: Text(local('Service recovery', 'بازیابی سرویس')),
              )
            else ...[
              Text(t('login.recovery.unix')),
              _technical(value('RecoveryUnixCommand')),
              Text(t('login.recovery.windows')),
              _technical(value('RecoveryWindowsCommand')),
              Text(t('login.recovery.result')),
            ],
          ],
        ),
    ]);
  }

  List<Widget> _messages() => [
    if (value('Notice').isNotEmpty)
      _banner(value('Notice'), CaspianStyle.yellow),
    if (flag('HasProblem'))
      Semantics(
        container: true,
        identifier: 'problem',
        liveRegion: true,
        child: _banner(
          [
            value('ProblemHeadline'),
            value('ProblemAdvice'),
            if (flag('Advanced')) value('ProblemDetail'),
          ].where((s) => s.isNotEmpty).join('\n'),
          CaspianStyle.coral,
        ),
      ),
    if (flag('ProblemCountry'))
      TextButton(
        onPressed: () => _jump('advanced'),
        child: Text(t('fault.countrymissing.action')),
      ),
  ];

  List<Widget> _dashboard() => [
    if (_showSetupChecklist)
      KeyedSubtree(
        key: _sections['setup'],
        child: ConnectionSetupChecklist(
          page: p,
          fa: fa,
          onNavigate: _jump,
          onFinish: _finishChecklist,
        ),
      ),
    KeyedSubtree(key: _sections['status'], child: _hero()),
    _tiles(),
    if (flag('TrafficCut')) _banner(t('cut.banner'), CaspianStyle.coral),
    ..._messages(),
    LayoutBuilder(
      builder: (_, constraints) {
        final wifi = KeyedSubtree(key: _sections['wifi'], child: _wifi());
        final rest = _stack([
          KeyedSubtree(key: _sections['config'], child: _config()),
          if (value('DetectedLine').isNotEmpty)
            _banner(value('DetectedLine'), CaspianStyle.surface),
          KeyedSubtree(key: _sections['events'], child: _events()),
        ]);
        if (constraints.maxWidth < 760) return _stack([wifi, rest]);
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: wifi),
            const SizedBox(width: 20),
            Expanded(child: rest),
          ],
        );
      },
    ),
    KeyedSubtree(key: _sections['connections'], child: _connections()),
    Center(
      child: OutlinedButton(
        onPressed: c.busy ? null : () => c.setAdvanced(!flag('Advanced')),
        child: Text(t(flag('Advanced') ? 'advanced.hide' : 'advanced.show')),
      ),
    ),
    KeyedSubtree(
      key: _sections['password'],
      child: _card(t('password.heading'), Icons.lock_outline, [
        Text(t('password.hint')),
        _ActionForm(
          controller: c,
          action: 'password',
          submit: t('password.save'),
          fields: [
            _Field('current', t('password.current'), secret: true),
            _Field('password', t('password.new'), secret: true),
            _Field('confirm', t('password.confirm'), secret: true),
          ],
        ),
      ]),
    ),
    KeyedSubtree(key: _sections['identifiers'], child: _identifierCard()),
    if (flag('Advanced'))
      KeyedSubtree(key: _sections['advanced'], child: _advanced()),
  ];

  Widget _hero() {
    final background = switch (value('HeroClass')) {
      'ok' => CaspianStyle.sage,
      'cut' => CaspianStyle.coral,
      'wait' => CaspianStyle.yellow,
      _ => CaspianStyle.coral,
    };
    final status = _stack([
      Text(
        _rows(p['Tiles']).firstOrNull?['Label'] ?? t('status.heading'),
        style: const TextStyle(fontWeight: FontWeight.w600),
      ),
      Semantics(
        liveRegion: true,
        child: Wrap(
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: 10,
          children: [
            Icon(
              flag('Connected')
                  ? Icons.radio_button_checked
                  : Icons.radio_button_unchecked,
              size: 19,
            ),
            Text(
              value('StatusWord'),
              style: const TextStyle(
                fontSize: 36,
                fontWeight: FontWeight.w700,
                height: 1.3,
              ),
            ),
          ],
        ),
      ),
    ], gap: 8);
    final notes = _stack([
      Align(
        alignment: AlignmentDirectional.centerStart,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 3),
          decoration: BoxDecoration(
            color: CaspianStyle.cyan,
            borderRadius: BorderRadius.circular(24),
          ),
          child: Text(
            value('NextLabel'),
            style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 14),
          ),
        ),
      ),
      Text(value('NextStep')),
      Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.shield_outlined, color: CaspianStyle.teal, size: 20),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              '${t('advanced.drop.label')}: ${t('advanced.drop.value')}',
              style: const TextStyle(fontSize: 14),
            ),
          ),
        ],
      ),
    ], gap: 10);
    final power = _stack([
      Center(
        child: SizedBox(
          width: 132,
          height: 132,
          child: Semantics(
            container: true,
            identifier: 'power',
            child: OutlinedButton(
              key: const ValueKey('power'),
              onPressed: c.busy
                  ? null
                  : () => c.act('power', {'on': flag('Running') ? '0' : '1'}),
              style: OutlinedButton.styleFrom(
                shape: const CircleBorder(),
                backgroundColor: background,
                foregroundColor: CaspianStyle.ink,
                side: const BorderSide(color: CaspianStyle.teal, width: 2),
                padding: const EdgeInsets.all(12),
              ),
              child: _stack([
                const Center(child: Icon(Icons.power_settings_new, size: 32)),
                Center(
                  child: Text(
                    t(flag('Running') ? 'power.off' : 'power.on'),
                    textAlign: TextAlign.center,
                  ),
                ),
              ], gap: 4),
            ),
          ),
        ),
      ),
      Text(
        c.desktop
            ? local(
                'Stops the hotspot and tunnel. This window stays open.',
                'هات‌اسپات و تونل را متوقف می‌کند. این پنجره باز می‌ماند.',
              )
            : local(
                'Stops the hotspot and tunnel. Devices using this Wi-Fi will lose access to Caspian until it starts again.',
                'هات‌اسپات و تونل را متوقف می‌کند. دستگاه‌های این وای‌فای تا راه‌اندازی دوباره به کاسپین دسترسی ندارند.',
              ),
        textAlign: TextAlign.center,
        style: const TextStyle(fontSize: 14),
      ),
    ], gap: 12);
    final cut = _stack([
      Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Semantics(
            container: true,
            identifier: 'traffic',
            label: t('cut.switchlabel'),
            child: Switch(
              key: const ValueKey('traffic'),
              value: !flag('TrafficCut'),
              activeTrackColor: CaspianStyle.sage,
              activeThumbColor: CaspianStyle.surface,
              trackOutlineColor: const WidgetStatePropertyAll(CaspianStyle.ink),
              onChanged: c.busy
                  ? null
                  : (enabled) => c.act('cut', {'cut': enabled ? '0' : '1'}),
            ),
          ),
          Flexible(
            child: Text(
              t('cut.switchlabel'),
              style: const TextStyle(fontWeight: FontWeight.w700),
            ),
          ),
        ],
      ),
      Text(
        t('cut.caption'),
        textAlign: TextAlign.center,
        style: const TextStyle(fontSize: 14),
      ),
    ], gap: 12);
    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: CaspianStyle.teal),
      ),
      child: LayoutBuilder(
        builder: (_, constraints) {
          if (constraints.maxWidth >= 1000) {
            return Row(
              children: [
                Expanded(flex: 2, child: status),
                const SizedBox(width: 24),
                Expanded(flex: 3, child: notes),
                const SizedBox(width: 24),
                Expanded(flex: 2, child: power),
                if (flag('Running')) ...[
                  const SizedBox(width: 20),
                  Expanded(flex: 2, child: cut),
                ],
              ],
            );
          }
          return _stack([
            status,
            notes,
            if (constraints.maxWidth >= 500)
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: power),
                  if (flag('Running')) ...[
                    const SizedBox(width: 24),
                    Expanded(child: cut),
                  ],
                ],
              )
            else ...[
              power,
              if (flag('Running')) cut,
            ],
          ]);
        },
      ),
    );
  }

  Widget _tiles() => LayoutBuilder(
    builder: (_, constraints) {
      final tiles = _rows(p['Tiles']).skip(1).toList();
      return Wrap(
        spacing: 16,
        runSpacing: 16,
        children: [
          for (var i = 0; i < tiles.length; i++)
            SizedBox(
              width: constraints.maxWidth < 600
                  ? constraints.maxWidth
                  : (constraints.maxWidth - 32) / 3,
              child: Card(
                clipBehavior: Clip.antiAlias,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Stack(
                      children: [
                        PositionedDirectional(
                          top: -8,
                          end: -8,
                          child: Icon(
                            [
                              Icons.devices,
                              Icons.key_outlined,
                              Icons.access_time,
                            ][i % 3],
                            size: 72,
                            color: CaspianStyle.sage,
                          ),
                        ),
                        Padding(
                          padding: const EdgeInsets.all(20),
                          child: _stack([
                            Text(
                              _str(tiles[i]['Label']),
                              style: const TextStyle(
                                fontSize: 14,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                            Text(
                              _str(tiles[i]['ValueLTR']).isNotEmpty
                                  ? _str(tiles[i]['ValueLTR'])
                                  : _str(tiles[i]['Value']),
                              textDirection:
                                  _str(tiles[i]['ValueLTR']).isNotEmpty
                                  ? TextDirection.ltr
                                  : null,
                              style: const TextStyle(
                                fontWeight: FontWeight.w700,
                                fontSize: 28,
                                height: 1.3,
                              ),
                            ),
                          ], gap: 8),
                        ),
                      ],
                    ),
                    Material(
                      color: CaspianStyle.ground,
                      child: InkWell(
                        onTap: () => _jump(['wifi', 'config', 'events'][i % 3]),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 20,
                            vertical: 10,
                          ),
                          child: Row(
                            mainAxisAlignment: MainAxisAlignment.end,
                            children: [
                              Flexible(
                                child: Text(
                                  t(
                                    [
                                      'wifi.heading',
                                      'config.heading',
                                      'events.heading',
                                    ][i % 3],
                                  ),
                                  style: const TextStyle(
                                    color: CaspianStyle.teal,
                                    fontWeight: FontWeight.w600,
                                    fontSize: 14,
                                  ),
                                ),
                              ),
                              const Icon(
                                Icons.chevron_right,
                                color: CaspianStyle.teal,
                                size: 18,
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      );
    },
  );

  Widget _wifi() => _card(t('wifi.heading'), Icons.wifi, [
    if (flag('HotspotReady')) ...[
      Center(
        child: Container(
          width: 228,
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            border: Border.all(color: CaspianStyle.edge),
            borderRadius: BorderRadius.circular(9),
          ),
          child: _stack([
            if (value('QR').isNotEmpty)
              RepaintBoundary(
                key: const ValueKey('wifi-qr'),
                child: SvgPicture.string(
                  value(
                    'QR',
                  ).replaceAll('class="qr-bg"', 'class="qr-bg" fill="#FFFFFF"'),
                  width: 200,
                  height: 200,
                  semanticsLabel: t('wifi.qr.caption'),
                ),
              )
            else
              Text(value('QRProblem')),
            Text(
              t('wifi.qr.caption'),
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 13, color: CaspianStyle.quiet),
            ),
          ], gap: 12),
        ),
      ),
      _credential(t('wifi.name'), value('SSID'), ltr: false),
      _credential(t('wifi.password'), value('Passphrase')),
      Text(
        value('DeviceLine'),
        style: const TextStyle(fontWeight: FontWeight.w700),
      ),
      const Divider(),
      Semantics(
        container: true,
        identifier: 'hotspot-expand',
        child: ExpansionTile(
          title: Text(t('wifi.change')),
          children: [_hotspotForm()],
        ),
      ),
    ] else ...[
      Text(t('wifi.intro')),
      _hotspotForm(),
    ],
  ]);

  Widget _hotspotForm() => _ActionForm(
    controller: c,
    action: 'hotspot',
    submit: t('wifi.form.save'),
    hint: t('wifi.form.hint'),
    fields: [
      _Field(
        'ssid',
        t('wifi.form.name'),
        initial: value('SSID').isEmpty ? value('SuggestedSSID') : value('SSID'),
        ltr: false,
      ),
      _Field(
        'passphrase',
        t('wifi.form.password'),
        initial: value('Passphrase').isEmpty
            ? value('SuggestedPassphrase')
            : value('Passphrase'),
      ),
    ],
  );

  Widget _config() => _card(t('config.heading'), Icons.key_outlined, [
    if (flag('HasConfig'))
      Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: CaspianStyle.ground,
          borderRadius: BorderRadius.circular(9),
        ),
        child: Wrap(
          spacing: 10,
          children: [
            Text(
              value('ConfigName'),
              style: const TextStyle(fontWeight: FontWeight.w700),
            ),
            _technical(value('ConfigSummary')),
          ],
        ),
      )
    else
      Text(t('config.none')),
    const Divider(),
    Semantics(
      container: true,
      identifier: 'config-expand',
      child: ExpansionTile(
        key: ValueKey('config-${flag('HasConfig')}'),
        initiallyExpanded: flag('SetupIncomplete'),
        title: Text(t(flag('HasConfig') ? 'config.replace' : 'config.add')),
        children: [
          _ActionForm(
            controller: c,
            action: 'config',
            submit: t('config.submit'),
            hint: t('config.paste.hint'),
            fields: [
              _Field(
                'config',
                t('config.paste.label'),
                lines: 4,
                placeholder: t('config.placeholder'),
                clearAfter: true,
              ),
              _Field('label', t('config.name.label'), ltr: false),
            ],
          ),
        ],
      ),
    ),
  ]);

  Widget _events() => _card(t('events.heading'), Icons.format_list_bulleted, [
    if (_rows(p['Events']).isEmpty)
      Text(t('events.empty'))
    else
      _eventRows(_rows(p['Events'])),
    Text(
      t('events.note'),
      style: const TextStyle(fontSize: 14, color: CaspianStyle.quiet),
    ),
  ]);

  Widget _eventRows(
    List<Map<String, dynamic>> rows, {
    bool technical = false,
  }) => Container(
    decoration: BoxDecoration(
      border: Border.all(color: CaspianStyle.edge),
      borderRadius: BorderRadius.circular(9),
    ),
    clipBehavior: Clip.antiAlias,
    child: Column(
      children: [
        for (var i = 0; i < rows.length; i++)
          Container(
            color: i.isEven ? CaspianStyle.ground : CaspianStyle.surface,
            padding: const EdgeInsets.all(12),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _technical(_str(rows[i]['At'])),
                const SizedBox(width: 12),
                Expanded(
                  child: technical
                      ? _technical(_str(rows[i]['Text']))
                      : Text(_str(rows[i]['Text'])),
                ),
              ],
            ),
          ),
      ],
    ),
  );

  List<_Field> _connectionFields() => [
    _Field(
      'internet_interface',
      t('advanced.internet'),
      initial: value('CurrentInternet'),
      options: {
        '': value('AutoInternet'),
        for (final i in _rows(p['Interfaces']))
          _str(i['Name']): '${i['Name']} - ${i['Kind']}',
      },
    ),
    _Field(
      'hotspot_interface',
      t('advanced.hotspotiface'),
      initial: value('CurrentHotspot'),
      options: {
        '': value('AutoHotspot'),
        for (final i in _rows(
          p['Interfaces'],
        ).where((i) => i['CanHost'] == true))
          _str(i['Name']): '${i['Name']} - ${i['Kind']}',
      },
    ),
    _Field(
      'band',
      t('advanced.band'),
      initial: value('CurrentBand'),
      options: {
        '': value('AutoBand'),
        for (final b in _rows(p['Bands'])) _str(b['Value']): _str(b['Words']),
      },
    ),
  ];

  Widget _connections() => _card(t('connections.heading'), Icons.lan_outlined, [
    Text(t('connections.hint')),
    _ActionForm(
      controller: c,
      action: 'advanced',
      submit: t('advanced.save'),
      hint: t('connections.apply'),
      extra: const {'connections_only': '1'},
      fields: _connectionFields(),
    ),
  ]);

  Widget _advanced() => _card(t('advanced.heading'), Icons.tune, [
    Text(t('advanced.hint')),
    _ActionForm(
      controller: c,
      action: 'advanced',
      submit: t('advanced.save'),
      fields: [
        ..._connectionFields(),
        _Field(
          'channel',
          t('advanced.channel'),
          initial: value('CurrentChannel'),
          options: {
            '': value('AutoChannel'),
            for (final ch in _rows(p['Channels']))
              _str(ch['Value']): _str(ch['Value']),
          },
        ),
        _Field(
          'country',
          t('advanced.country'),
          initial: value('CurrentCountry'),
          placeholder: value('PlaceCountry'),
          hint: t('advanced.country.hint'),
        ),
        _Field(
          'subnet',
          t('advanced.subnet'),
          initial: value('CurrentSubnet'),
          placeholder: value('PlaceSubnet'),
          hint: t('advanced.subnet.hint'),
        ),
        _Field(
          'engine_log_level',
          t('advanced.loglevel'),
          initial: _str(
            _rows(
              p['LogLevels'],
            ).where((l) => l['Selected'] == true).firstOrNull?['Value'],
          ),
          options: {
            '': value('AutoLogLevel'),
            for (final l in _rows(p['LogLevels']))
              _str(l['Value']): _str(l['Value']),
          },
          hint: t('advanced.loglevel.hint'),
        ),
        _Field(
          'panel_on_lan',
          t('advanced.panelonlan'),
          checkbox: true,
          initial: flag('PanelOnLAN') ? '1' : '',
          hint: t('advanced.panelonlan.hint'),
        ),
      ],
    ),
    if (flag('ChannelPinned')) Text(t('advanced.channelpinned')),
    const Divider(),
    Text(
      t('advanced.recover.heading'),
      style: Theme.of(context).textTheme.titleMedium,
    ),
    Text(t('advanced.recover.hint')),
    FilledButton(
      onPressed: c.busy ? null : () => c.act('recover', {}),
      child: Text(t('advanced.recover.button')),
    ),
    _facts('advanced.fixed.heading', _rows(p['FixedFacts'])),
    if (_rows(p['ConfigFacts']).isNotEmpty) ...[
      _facts('advanced.config.heading', _rows(p['ConfigFacts'])),
      Text(t('advanced.nosecrets')),
    ],
    Text(
      t('advanced.log.heading'),
      style: Theme.of(context).textTheme.titleMedium,
    ),
    Text(value('EnginePhase')),
    if (value('EngineReason').isNotEmpty) _technical(value('EngineReason')),
    if (_rows(p['EngineLog']).isEmpty)
      Text(t('advanced.log.empty'))
    else
      _eventRows(_rows(p['EngineLog']), technical: true),
  ]);

  Widget _facts(String heading, List<Map<String, dynamic>> facts) => _stack([
    Text(t(heading), style: Theme.of(context).textTheme.titleMedium),
    for (final f in facts)
      _stack([
        Text(
          _str(f['Label']),
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
        if (_str(f['Words']).isNotEmpty)
          Text(_str(f['Words']))
        else
          _technical(_str(f['Value'])),
      ], gap: 4),
  ], gap: 12);

  Widget _identifierCard() =>
      _card(t('identifiers.heading'), Icons.fingerprint, [
        Text(t('identifiers.hint')),
        for (final field in ['uuid', 'imei'])
          _stack([
            Text(
              t('identifiers.$field'),
              style: const TextStyle(fontWeight: FontWeight.w600),
            ),
            Row(
              children: [
                Expanded(
                  child: _technical(
                    _str(_identifiers[field]).isEmpty
                        ? t('identifiers.empty')
                        : _str(_identifiers[field]),
                  ),
                ),
                IconButton(
                  tooltip: t('identifiers.copy'),
                  onPressed: _str(_identifiers[field]).isEmpty
                      ? null
                      : () => Clipboard.setData(
                          ClipboardData(text: _str(_identifiers[field])),
                        ),
                  icon: const Icon(Icons.copy),
                ),
              ],
            ),
          ], gap: 4),
        FilledButton(
          onPressed: c.busy
              ? null
              : () async {
                  try {
                    final generated = await c.identifiers();
                    if (mounted && c.state['view'] == 'dashboard') {
                      setState(() => _identifiers = generated);
                    }
                  } catch (_) {
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(t('identifiers.failed'))),
                      );
                    }
                  }
                },
          child: Text(t('identifiers.generate')),
        ),
        Text(t('identifiers.warning')),
      ]);

  List<Widget> _helpCards() => [
    _card(t('help.what.heading'), Icons.help_outline, [
      Text(t('help.what.body')),
      Wrap(
        spacing: 12,
        runSpacing: 12,
        children: [
          for (final k in ['device', 'box', 'tunnel', 'server'])
            Chip(
              label: Text(t('help.flow.$k')),
              backgroundColor: k == 'box'
                  ? CaspianStyle.sage
                  : CaspianStyle.ground,
            ),
        ],
      ),
      Text(t('help.flow.note')),
    ]),
    _card(t('help.arrange.heading'), Icons.lan_outlined, [
      Text(t('help.arrange.body')),
      for (final arrangement in ['a', 'b']) ...[
        Text(
          t('help.arrange.$arrangement.heading'),
          style: Theme.of(context).textTheme.titleMedium,
        ),
        Text(t('help.arrange.$arrangement.top')),
        Text(t('help.arrange.$arrangement.bottom')),
        Text(t('help.arrange.$arrangement.when')),
      ],
      Text(t('help.arrange.nounplug')),
    ]),
    _card(t('help.two.heading'), Icons.power_settings_new, [
      for (final k in [
        'intro',
        'power.heading',
        'power.body',
        'cut.heading',
        'cut.body',
        'lockout',
        'which',
        'restart',
      ])
        Text(t('help.two.$k')),
    ]),
    _card(t('help.controls.heading'), Icons.tune, [
      for (final key in [
        'switch',
        'cut',
        'config',
        'wifi',
        'uplink',
        'hotspotiface',
        'channel',
        'country',
        'subnet',
        'panelonlan',
      ])
        Text(t('help.controls.$key')),
    ]),
    _card(t('help.security.heading'), Icons.shield_outlined, [
      for (final key in [
        'does.heading',
        'does.failclosed',
        'does.dns',
        'does.egress',
        'does.isolation',
        'does.secrets',
        'does.giveback',
        'not.heading',
        'not.intro',
        'not.doh',
        'not.ipv6',
        'not.ownbox',
        'not.endpoints',
        'not.wifi',
      ])
        Text(t('help.security.$key')),
    ]),
    _card(t('help.trouble.heading'), Icons.info_outline, [
      for (final topic in [
        'nossid',
        'nointernet',
        'slow',
        'lostpanel',
        'forgot',
      ]) ...[
        Text(
          t('help.trouble.$topic.q'),
          style: const TextStyle(fontWeight: FontWeight.w700),
        ),
        Text(t('help.trouble.$topic.a')),
      ],
    ]),
    OutlinedButton(
      onPressed: () => _jump('status'),
      child: Text(t('nav.back')),
    ),
  ];

  Future<void> _serviceDialog() => showDialog<void>(
    context: context,
    builder: (dialogContext) => AnimatedBuilder(
      animation: c,
      builder: (_, _) => AlertDialog(
        title: Text(local('Service recovery', 'بازیابی سرویس')),
        content: SizedBox(
          width: 440,
          child: SingleChildScrollView(
            child: _stack([
              Text(
                local(
                  'Manage Caspian on this computer. Administrator access may be requested by the operating system.',
                  'کاسپین را در این رایانه مدیریت کنید. سیستم‌عامل ممکن است دسترسی مدیر را درخواست کند.',
                ),
              ),
              if (c.serviceMessage != null) _technical(c.serviceMessage!),
              if (c.busy) const LinearProgressIndicator(),
              for (final action in [
                ('install', local('Install services', 'نصب سرویس‌ها')),
                ('start', local('Start services', 'راه‌اندازی سرویس‌ها')),
                (
                  'restart',
                  local('Restart services', 'راه‌اندازی دوباره سرویس‌ها'),
                ),
                ('stop', local('Stop services', 'توقف سرویس‌ها')),
                ('reset-password', local('Reset password', 'بازنشانی گذرواژه')),
              ])
                if (c.supportedServiceActions.contains(action.$1))
                  OutlinedButton(
                    onPressed: c.busy ? null : () => c.serviceAction(action.$1),
                    child: Text(action.$2),
                  ),
            ]),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: Text(local('Close', 'بستن')),
          ),
        ],
      ),
    ),
  );

  Widget _credential(String label, String text, {bool ltr = true}) => _stack([
    Text(
      label,
      style: const TextStyle(
        fontWeight: FontWeight.w600,
        color: CaspianStyle.quiet,
        fontSize: 14,
      ),
    ),
    Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: CaspianStyle.ground,
        borderRadius: BorderRadius.circular(9),
      ),
      child: SelectableText(
        text,
        textDirection: ltr ? TextDirection.ltr : null,
        style: const TextStyle(
          fontFamily: 'CaspianMono',
          fontFamilyFallback: ['Vazirmatn', 'CaspianSans'],
          fontSize: 18,
        ),
      ),
    ),
  ], gap: 8);

  Widget _technical(String text) => SelectableText(
    text,
    textDirection: TextDirection.ltr,
    style: const TextStyle(
      fontFamily: 'CaspianMono',
      fontFamilyFallback: ['Vazirmatn', 'CaspianSans'],
      color: CaspianStyle.quiet,
      fontSize: 14,
    ),
  );
  Widget _banner(String text, Color color) => Container(
    width: double.infinity,
    padding: const EdgeInsets.all(16),
    decoration: BoxDecoration(
      color: color,
      border: Border.all(color: CaspianStyle.edge),
      borderRadius: BorderRadius.circular(9),
    ),
    child: Text(text),
  );
  Widget _card(String title, IconData icon, List<Widget> children) => Card(
    clipBehavior: Clip.antiAlias,
    child: DecoratedBox(
      decoration: const BoxDecoration(
        border: Border(top: BorderSide(color: CaspianStyle.cyan, width: 3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.all(20),
            child: Row(
              children: [
                Icon(icon, size: 22, color: CaspianStyle.teal),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    title,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                ),
              ],
            ),
          ),
          const Divider(height: 1),
          Padding(padding: const EdgeInsets.all(20), child: _stack(children)),
        ],
      ),
    ),
  );
}

Widget _stack(List<Widget> children, {double gap = 20}) => Column(
  crossAxisAlignment: CrossAxisAlignment.stretch,
  children: [
    for (var i = 0; i < children.length; i++) ...[
      if (i > 0) SizedBox(height: gap),
      children[i],
    ],
  ],
);

class _Field {
  const _Field(
    this.name,
    this.label, {
    this.initial = '',
    this.placeholder,
    this.hint,
    this.secret = false,
    this.clearAfter = false,
    this.ltr = true,
    this.lines = 1,
    this.options,
    this.checkbox = false,
  });
  final String name, label, initial;
  final String? placeholder, hint;
  final bool secret, clearAfter, ltr, checkbox;
  final int lines;
  final Map<String, String>? options;
}

class _ActionForm extends StatefulWidget {
  const _ActionForm({
    super.key,
    required this.controller,
    required this.action,
    required this.submit,
    required this.fields,
    this.hint,
    this.extra = const {},
  });
  final CaspianController controller;
  final String action, submit;
  final String? hint;
  final List<_Field> fields;
  final Map<String, String> extra;
  @override
  State<_ActionForm> createState() => _ActionFormState();
}

class _ActionFormState extends State<_ActionForm> {
  final _text = <String, TextEditingController>{};
  final _values = <String, String>{};
  @override
  void initState() {
    super.initState();
    for (final field in widget.fields) {
      _text[field.name] = TextEditingController(text: field.initial);
      _values[field.name] = field.initial;
    }
  }

  @override
  void dispose() {
    for (final controller in _text.values) {
      controller.dispose();
    }
    super.dispose();
  }

  Future<void> _submit() async {
    if (widget.controller.busy) return;
    final values = {...widget.extra};
    for (final field in widget.fields) {
      if (field.checkbox) {
        if (_values[field.name] == '1') values[field.name] = '1';
      } else {
        values[field.name] = field.options == null
            ? _text[field.name]!.text
            : (_values[field.name] ?? '');
      }
    }
    await widget.controller.act(widget.action, values);
    if (!mounted) return;
    if (widget.controller.state['ok'] == true) {
      for (final field in widget.fields.where(
        (f) => f.secret || f.clearAfter,
      )) {
        _text[field.name]?.clear();
      }
    }
  }

  @override
  Widget build(BuildContext context) => AutofillGroup(
    child: _stack([
      for (final field in widget.fields)
        _stack([
          if (field.checkbox)
            CheckboxListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(field.label),
              value: _values[field.name] == '1',
              onChanged: widget.controller.busy && !widget.controller.refreshing
                  ? null
                  : (checked) => setState(
                      () => _values[field.name] = checked == true ? '1' : '',
                    ),
            )
          else ...[
            Text(
              field.label,
              style: const TextStyle(
                fontWeight: FontWeight.w600,
                color: CaspianStyle.quiet,
              ),
            ),
            if (field.options != null)
              DropdownButtonFormField<String>(
                key: ValueKey('${widget.action}-${field.name}'),
                initialValue: field.options!.containsKey(_values[field.name])
                    ? _values[field.name]
                    : '',
                isExpanded: true,
                items: field.options!.entries
                    .map(
                      (e) => DropdownMenuItem(
                        value: e.key,
                        child: Text(
                          e.value,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    )
                    .toList(),
                onChanged:
                    widget.controller.busy && !widget.controller.refreshing
                    ? null
                    : (value) =>
                          setState(() => _values[field.name] = value ?? ''),
              )
            else
              Semantics(
                container: true,
                identifier: '${widget.action}-${field.name}',
                label: field.label,
                child: TextField(
                  key: ValueKey('${widget.action}-${field.name}'),
                  controller: _text[field.name],
                  enabled:
                      !widget.controller.busy || widget.controller.refreshing,
                  obscureText: field.secret,
                  textDirection: field.ltr ? TextDirection.ltr : null,
                  minLines: field.lines,
                  maxLines: field.lines,
                  enableSuggestions: false,
                  autocorrect: false,
                  decoration: InputDecoration(hintText: field.placeholder),
                  autofillHints: field.secret
                      ? [
                          field.name == 'current' || widget.action == 'login'
                              ? AutofillHints.password
                              : AutofillHints.newPassword,
                        ]
                      : null,
                  onSubmitted:
                      field.secret &&
                          (widget.action == 'login' || field.name == 'confirm')
                      ? (_) => _submit()
                      : null,
                ),
              ),
          ],
          if (field.hint != null && field.hint!.isNotEmpty)
            Text(
              field.hint!,
              style: const TextStyle(fontSize: 14, color: CaspianStyle.quiet),
            ),
        ], gap: 8),
      if (widget.hint != null)
        Text(
          widget.hint!,
          style: const TextStyle(fontSize: 14, color: CaspianStyle.quiet),
        ),
      Align(
        alignment: AlignmentDirectional.centerStart,
        child: Semantics(
          container: true,
          identifier: '${widget.action}-submit',
          child: FilledButton(
            key: ValueKey('${widget.action}-submit'),
            onPressed: widget.controller.busy ? null : _submit,
            child: Text(widget.submit),
          ),
        ),
      ),
    ], gap: 16),
  );
}
