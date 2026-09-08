// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'controller.dart';
import 'theme.dart';

class ServiceOnboarding extends StatefulWidget {
  const ServiceOnboarding({
    super.key,
    required this.controller,
    required this.onContinue,
    required this.onRecovery,
  });
  final CaspianController controller;
  final VoidCallback onContinue;
  final VoidCallback onRecovery;

  @override
  State<ServiceOnboarding> createState() => _ServiceOnboardingState();
}

class _ServiceOnboardingState extends State<ServiceOnboarding> {
  bool _saved = false;
  String _lastAction = 'install';
  String? _lastPassword;
  CaspianController get c => widget.controller;
  bool get fa => c.language == 'fa';
  bool get mac => defaultTargetPlatform == TargetPlatform.macOS;
  bool get windows => defaultTargetPlatform == TargetPlatform.windows;
  String words(String en, String persian) => fa ? persian : en;

  Future<void> _run(String action) async {
    setState(() {
      _lastAction = action;
      _saved = false;
    });
    await c.setUpService(action);
  }

  @override
  Widget build(BuildContext context) {
    if (_lastPassword != c.setupPassword) {
      _lastPassword = c.setupPassword;
      _saved = false;
    }
    final phase = c.setupPhase;
    final active =
        phase == ServiceSetupPhase.installing ||
        phase == ServiceSetupPhase.checking;
    final ready =
        phase == ServiceSetupPhase.ready ||
        (phase == ServiceSetupPhase.idle &&
            {'login', 'setup'}.contains(c.state['view']));
    final failed = phase == ServiceSetupPhase.failed;
    final hasPassword = c.setupPassword?.isNotEmpty == true;
    final needsPassword = c.state['view'] == 'setup';
    final saveRequired = hasPassword || (windows && !needsPassword);
    return _SetupCard(
      title: words('Set up Caspian', 'راه‌اندازی کاسپین'),
      icon: Icons.shield_outlined,
      children: [
        if (!ready)
          Text(
            words(
              'Welcome. Caspian needs its background services to connect your tunnel and share WiFi. This window stays open during setup.',
              'خوش آمدید. کاسپین برای اتصال تونل و اشتراک وای‌فای به سرویس‌های پس‌زمینه نیاز دارد. این پنجره هنگام راه‌اندازی باز می‌ماند.',
            ),
          ),
        if (mac && !ready)
          Text(
            words(
              'Copy Caspian to Applications, then open it from Applications before you continue.',
              'کاسپین را به پوشهٔ Applications کپی کنید و پیش از ادامه، آن را از همان پوشه باز کنید.',
            ),
          ),
        if (!ready)
          Text(
            mac
                ? words(
                    'macOS will ask for your Mac administrator password to install the services. Your Caspian sign-in password is separate. Enter the Mac password only in the macOS prompt.',
                    'macOS برای نصب سرویس‌ها گذرواژهٔ مدیر مک را درخواست می‌کند. گذرواژهٔ ورود به کاسپین جداست. گذرواژهٔ مک را فقط در پنجرهٔ macOS وارد کنید.',
                  )
                : windows
                ? words(
                    'Windows will ask you to approve changes to this computer. Caspian Setup installs the background services. Keep the Caspian password you chose in Setup; a new installation or repair may show a new password in a Windows window.',
                    'ویندوز تأیید تغییرات در این رایانه را درخواست می‌کند. نصب‌کنندهٔ کاسپین سرویس‌های پس‌زمینه را نصب می‌کند. گذرواژه‌ای را که در نصب‌کننده انتخاب کرده‌اید نگه دارید؛ نصب یا تعمیر ممکن است گذرواژهٔ تازه‌ای در پنجرهٔ ویندوز نشان دهد.',
                  )
                : words(
                    'Your system may ask for administrator approval to install the background services. The administrator password and your Caspian sign-in password are separate.',
                    'سیستم ممکن است برای نصب سرویس‌های پس‌زمینه تأیید مدیر را درخواست کند. گذرواژهٔ مدیر با گذرواژهٔ ورود به کاسپین متفاوت است.',
                  ),
          ),
        if (active)
          Semantics(
            container: true,
            identifier: 'onboarding-progress',
            liveRegion: true,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const LinearProgressIndicator(),
                const SizedBox(height: 12),
                Text(
                  phase == ServiceSetupPhase.installing
                      ? words(
                          'Waiting for system approval and service setup. Complete any system prompt to continue.',
                          'در انتظار تأیید سیستم و نصب سرویس‌ها. برای ادامه، پیام سیستم را پاسخ دهید.',
                        )
                      : words(
                          'Waiting for Caspian to respond.',
                          'در انتظار پاسخ کاسپین.',
                        ),
                ),
              ],
            ),
          ),
        if (failed)
          Semantics(
            container: true,
            identifier: 'onboarding-error',
            liveRegion: true,
            child: Text(
              c.setupError ??
                  words(
                    'Setup has not finished. Try again or open service recovery.',
                    'راه‌اندازی کامل نشده است. دوباره تلاش کنید یا بازیابی سرویس را باز کنید.',
                  ),
            ),
          ),
        if (ready)
          Semantics(
            liveRegion: true,
            child: Text(
              needsPassword
                  ? words(
                      'Caspian is ready. Choose your Caspian password to continue.',
                      'کاسپین آماده است. برای ادامه، گذرواژهٔ کاسپین را انتخاب کنید.',
                    )
                  : words(
                      'Caspian is ready for sign-in. Sign in to configure your connection.',
                      'کاسپین برای ورود آماده است. برای تنظیم اتصال وارد شوید.',
                    ),
            ),
          ),
        if (hasPassword) ...[
          Text(
            words(
              'Save your Caspian password',
              'گذرواژهٔ کاسپین را ذخیره کنید',
            ),
            style: Theme.of(context).textTheme.titleMedium,
          ),
          Text(
            words(
              'Save this password somewhere private. You will use it to sign in to Caspian.',
              'این گذرواژه را در جایی خصوصی ذخیره کنید. برای ورود به کاسپین به آن نیاز دارید.',
            ),
          ),
          Semantics(
            container: true,
            identifier: 'onboarding-password',
            child: SelectableText(
              c.setupPassword!,
              key: const ValueKey('onboarding-password'),
              textDirection: TextDirection.ltr,
              style: const TextStyle(
                fontFamily: 'CaspianMono',
                fontFamilyFallback: ['Vazirmatn', 'CaspianSans'],
                fontSize: 18,
              ),
            ),
          ),
        ],
        if (ready && !hasPassword && !needsPassword)
          Text(
            windows
                ? words(
                    'Use the Caspian password you chose in Setup, or the new password shown by Windows during installation or repair.',
                    'از گذرواژهٔ کاسپین که در نصب‌کننده انتخاب کردید، یا گذرواژهٔ تازه‌ای که ویندوز هنگام نصب یا تعمیر نشان داد استفاده کنید.',
                  )
                : words(
                    'Use your existing Caspian password. If you have not set one yet, the next screen will ask you to choose it.',
                    'از گذرواژهٔ فعلی کاسپین استفاده کنید. اگر هنوز گذرواژه‌ای تنظیم نکرده‌اید، در صفحهٔ بعد آن را انتخاب می‌کنید.',
                  ),
          ),
        if (ready && saveRequired)
          Semantics(
            container: true,
            identifier: 'onboarding-saved',
            child: CheckboxListTile(
              key: const ValueKey('onboarding-saved'),
              contentPadding: EdgeInsets.zero,
              controlAffinity: ListTileControlAffinity.leading,
              title: Text(
                hasPassword
                    ? words(
                        'I have saved my Caspian password',
                        'گذرواژهٔ کاسپین را ذخیره کرده‌ام',
                      )
                    : words(
                        'I have my Caspian password',
                        'گذرواژهٔ کاسپین را دارم',
                      ),
              ),
              value: _saved,
              onChanged: c.busy
                  ? null
                  : (value) => setState(() => _saved = value == true),
            ),
          ),
        if (ready)
          _button(
            'onboarding-continue',
            needsPassword
                ? words(
                    'Continue to set a password',
                    'ادامه برای تنظیم گذرواژه',
                  )
                : words('Continue to sign in', 'ادامه برای ورود'),
            c.busy || (saveRequired && !_saved)
                ? null
                : () {
                    c.acknowledgeSetupPassword();
                    widget.onContinue();
                  },
          ),
        if (!active && !ready) ...[
          if (failed)
            _button(
              'onboarding-retry',
              c.setupNeedsReadinessRetry
                  ? words('Check again', 'بررسی دوباره')
                  : words('Try setup again', 'تلاش دوباره برای راه‌اندازی'),
              c.busy
                  ? null
                  : c.setupNeedsReadinessRetry
                  ? c.retrySetupCheck
                  : () => _run(_lastAction),
            )
          else if (c.supportedServiceActions.contains('install'))
            _button(
              'onboarding-install',
              words('Install and continue', 'نصب و ادامه'),
              c.busy ? null : () => _run('install'),
            ),
          Text(
            words(
              'Already installed on this computer?',
              'قبلاً روی این رایانه نصب شده است؟',
            ),
          ),
          _button(
            'onboarding-start',
            words('Start existing services', 'راه‌اندازی سرویس‌های نصب‌شده'),
            c.busy ? null : () => _run('start'),
            primary: false,
          ),
        ],
        _button(
          'onboarding-recovery',
          words('Service recovery', 'بازیابی سرویس'),
          active ? null : widget.onRecovery,
          primary: false,
        ),
      ],
    );
  }

  Widget _button(
    String id,
    String label,
    VoidCallback? action, {
    bool primary = true,
  }) => Align(
    alignment: AlignmentDirectional.centerStart,
    child: Semantics(
      container: true,
      identifier: id,
      child: primary
          ? FilledButton(
              key: ValueKey(id),
              onPressed: action,
              child: Text(label),
            )
          : OutlinedButton(
              key: ValueKey(id),
              onPressed: action,
              child: Text(label),
            ),
    ),
  );
}

class ConnectionSetupChecklist extends StatelessWidget {
  const ConnectionSetupChecklist({
    super.key,
    required this.page,
    required this.fa,
    required this.onNavigate,
    required this.onFinish,
  });
  final Map<String, dynamic> page;
  final bool fa;
  final ValueChanged<String> onNavigate;
  final VoidCallback onFinish;
  String words(String en, String persian) => fa ? persian : en;

  @override
  Widget build(BuildContext context) {
    final config = page['HasConfig'] == true;
    final hotspot = page['HotspotReady'] == true;
    final connected =
        page['Running'] == true &&
        page['Connected'] == true &&
        page['TrafficCut'] != true &&
        page['HasProblem'] != true;
    final joined =
        (page['DeviceCount'] is num ? page['DeviceCount'] as num : 0) > 0;
    final ready = config && hotspot && connected && joined;
    return _SetupCard(
      key: const ValueKey('setup-checklist'),
      title: words(
        'Finish setting up Caspian',
        'راه‌اندازی کاسپین را کامل کنید',
      ),
      icon: Icons.checklist,
      children: [
        Text(
          words(
            'Complete these steps to share your connection with another device.',
            'برای اشتراک اتصال با دستگاهی دیگر، این مراحل را انجام دهید.',
          ),
        ),
        _step(
          'config',
          'config',
          config,
          words('Add your config', 'افزودن پیکربندی'),
          words(
            'Paste the configuration supplied by your provider.',
            'پیکربندی دریافتی از ارائه‌دهنده را وارد کنید.',
          ),
        ),
        _step(
          'hotspot',
          'wifi',
          hotspot,
          words(
            'Choose WiFi name and password',
            'انتخاب نام و گذرواژهٔ وای‌فای',
          ),
          words(
            'Save the WiFi details that your other devices will use.',
            'مشخصات وای‌فای را برای اتصال دستگاه‌های دیگر ذخیره کنید.',
          ),
        ),
        _step(
          'start',
          'status',
          connected,
          words('Switch on Caspian', 'روشن کردن کاسپین'),
          connected
              ? words(
                  'The tunnel is connected and client traffic is enabled.',
                  'تونل متصل است و ترافیک دستگاه‌ها فعال است.',
                )
              : words(
                  'Use the power control, then wait for a connected status. Resolve any problem shown there.',
                  'از دکمهٔ روشن کردن استفاده کنید و منتظر وضعیت متصل بمانید. هر مشکل نمایش‌داده‌شده را برطرف کنید.',
                ),
        ),
        _step(
          'join',
          'wifi',
          joined,
          words('Join from another device', 'اتصال از دستگاهی دیگر'),
          joined
              ? words(
                  'Caspian reports a device on its WiFi.',
                  'کاسپین یک دستگاه متصل به وای‌فای را گزارش می‌کند.',
                )
              : words(
                  'On your phone or another device, scan the WiFi code or enter the WiFi name and password.',
                  'با تلفن یا دستگاهی دیگر، کد وای‌فای را اسکن کنید یا نام و گذرواژهٔ وای‌فای را وارد کنید.',
                ),
        ),
        if (ready)
          Semantics(
            container: true,
            identifier: 'checklist-ready',
            liveRegion: true,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  words(
                    'Your connection is ready and a device has joined.',
                    'اتصال شما آماده است و یک دستگاه متصل شده است.',
                  ),
                ),
                const SizedBox(height: 12),
                FilledButton(
                  key: const ValueKey('checklist-ready'),
                  onPressed: onFinish,
                  child: Text(words('Continue to Caspian', 'ادامه به کاسپین')),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Widget _step(
    String id,
    String section,
    bool done,
    String title,
    String description,
  ) => Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Padding(
        padding: const EdgeInsets.only(top: 10),
        child: Icon(
          done ? Icons.check_circle : Icons.radio_button_unchecked,
          color: CaspianStyle.teal,
          semanticLabel: done
              ? words('Complete', 'کامل')
              : words('To do', 'انجام‌نشده'),
        ),
      ),
      const SizedBox(width: 12),
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Semantics(
              container: true,
              identifier: 'checklist-$id',
              child: TextButton(
                key: ValueKey('checklist-$id'),
                onPressed: () => onNavigate(section),
                child: Text(title),
              ),
            ),
            Text(description),
          ],
        ),
      ),
    ],
  );
}

class _SetupCard extends StatelessWidget {
  const _SetupCard({
    super.key,
    required this.title,
    required this.icon,
    required this.children,
  });
  final String title;
  final IconData icon;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Icon(icon, color: CaspianStyle.teal),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  title,
                  style: Theme.of(context).textTheme.titleLarge,
                ),
              ),
            ],
          ),
          for (final child in children) ...[const SizedBox(height: 16), child],
        ],
      ),
    ),
  );
}
