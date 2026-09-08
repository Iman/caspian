// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:async';
import 'package:flutter/foundation.dart';
import 'api_client.dart';
import 'lifecycle.dart';

enum ServiceSetupPhase { idle, installing, checking, ready, failed }

class CaspianController extends ChangeNotifier {
  CaspianController({
    ApiClient? client,
    DesktopLifecycle? lifecycle,
    bool? desktop,
    ApiClient Function()? setupClientFactory,
    this.setupReadinessAttempts = 10,
    this.setupRetryDelay = const Duration(seconds: 1),
    this.setupRequestTimeout = const Duration(seconds: 3),
  }) : _client = client ?? ApiClient(),
       _lifecycle = lifecycle ?? DesktopLifecycle(),
       desktop = desktop ?? !kIsWeb,
       _setupClientFactory = setupClientFactory,
       assert(setupReadinessAttempts > 0);
  ApiClient _client;
  final DesktopLifecycle _lifecycle;
  final bool desktop;
  final ApiClient Function()? _setupClientFactory;
  final int setupReadinessAttempts;
  final Duration setupRetryDelay;
  final Duration setupRequestTimeout;
  ApiClient? _setupClient;
  ServiceSetupPhase setupPhase = ServiceSetupPhase.idle;
  String? setupError;
  String? setupPassword;
  bool setupPasswordAcknowledged = false;
  bool _setupHelperCompleted = false;
  bool get setupNeedsReadinessRetry =>
      setupPhase == ServiceSetupPhase.failed && _setupHelperCompleted;
  Map<String, dynamic> state = {};
  bool busy = false;
  bool _refreshing = false;
  bool get refreshing => _refreshing;
  String? error;
  String? serviceMessage;
  String language = 'en';
  bool advanced = false;
  Timer? _timer;
  bool _disposed = false;

  List<String> get supportedServiceActions =>
      !desktop ? [] : ['install', 'start', 'stop', 'restart', 'reset-password'];

  void start() {
    unawaited(refresh());
    _timer ??= Timer.periodic(const Duration(seconds: 5), (_) {
      // Auth pages keep validation feedback until the next explicit action.
      if (!busy &&
          setupPhase != ServiceSetupPhase.failed &&
          !{'login', 'setup'}.contains(state['view'])) {
        unawaited(refresh());
      }
    });
  }

  String get _unavailable => language == 'fa'
      ? 'ارتباط با سرویس کاسپین برقرار نشد. دوباره تلاش کنید.'
      : 'Cannot reach the Caspian service. Try again.';

  Future<void> _update(
    Future<Map<String, dynamic>> Function() request, {
    bool readOnly = false,
  }) async {
    if (busy || _disposed) return;
    busy = true;
    _refreshing = readOnly;
    notifyListeners();
    try {
      Map<String, dynamic> next;
      try {
        next = await request();
      } on ApiSessionExpired {
        // Reopen login without retrying the mutation that was refused.
        next = await _client.request(
          '/api/v1/state',
          language: language,
          advanced: advanced,
        );
      }
      if (_disposed) return;
      state = next;
      error = null;
    } catch (_) {
      if (_disposed) return;
      // Discard old connected state and credentials when it cannot be checked.
      state = {};
      error = _unavailable;
    } finally {
      busy = false;
      _refreshing = false;
      if (!_disposed) notifyListeners();
    }
  }

  Future<void> refresh() => _update(
    () => _client.request(
      '/api/v1/state',
      language: language,
      advanced: advanced,
    ),
    readOnly: true,
  );

  Future<void> act(String action, Map<String, String> values) async {
    const actions = {
      'setup',
      'login',
      'power',
      'cut',
      'config',
      'hotspot',
      'advanced',
      'recover',
      'password',
      'logout',
    };
    if (!actions.contains(action)) throw ArgumentError.value(action, 'action');
    final page = state['page'] as Map? ?? {};
    final form = {...values, 'csrf': page['CSRF']?.toString() ?? ''};
    await _update(
      () => _client.request(
        '/api/v1/$action',
        form: form,
        language: language,
        advanced: advanced,
      ),
    );
  }

  Future<void> setLanguage(String value) async {
    if (busy || !{'en', 'fa'}.contains(value)) return;
    language = value;
    await refresh();
  }

  Future<void> setAdvanced(bool value) async {
    if (busy) return;
    advanced = value;
    await refresh();
  }

  Future<Map<String, dynamic>> identifiers() =>
      _client.request('/identifiers.json', language: language);

  Future<void> recoverService() => serviceAction('start');

  /// Explicit installation/start followed by a bounded service readiness check.
  /// Approval and installation share a phase: the launcher cannot distinguish them.
  Future<void> setUpService(String action) async {
    if (!{'install', 'start'}.contains(action)) {
      throw ArgumentError.value(action, 'action');
    }
    await _runSetup(action);
  }

  Future<void> retrySetupCheck() async {
    if (!setupNeedsReadinessRetry) return;
    await _runSetup(null);
  }

  Future<void> _runSetup(String? action) async {
    if (!desktop || busy || _disposed) return;
    busy = true;
    if (action != null) _setupHelperCompleted = false;
    setupPhase = action == null
        ? ServiceSetupPhase.checking
        : ServiceSetupPhase.installing;
    setupError = null;
    notifyListeners();
    ApiClient? probe;
    try {
      if (action != null) {
        final result = await _lifecycle.run(action);
        if (_disposed) return;
        _setupHelperCompleted = true;
        _rememberSetupPassword(result);
      }
      setupPhase = ServiceSetupPhase.checking;
      notifyListeners();
      // Keep probe cookies separate until a valid authentication page is ready.
      // Closing a failed native probe aborts its outstanding HTTP request, so a
      // late response cannot rotate the active client's form/session cookies.
      final candidate =
          _setupClientFactory?.call() ?? ApiClient(origin: _client.origin);
      probe = candidate;
      _setupClient = candidate;
      for (var attempt = 0; attempt < setupReadinessAttempts; attempt++) {
        try {
          final next = await candidate
              .request('/api/v1/state', language: language, advanced: advanced)
              .timeout(setupRequestTimeout);
          if (_disposed) return;
          if (!{'login', 'setup', 'dashboard'}.contains(next['view']) ||
              next['page'] is! Map ||
              next['strings'] is! Map) {
            throw const FormatException('Service is not ready');
          }
          _client.close();
          _client = candidate;
          probe = null;
          _setupClient = null;
          state = next;
          error = null;
          setupPhase = ServiceSetupPhase.ready;
          return;
        } on TimeoutException {
          // Stop on a timed-out request instead of overlapping another read.
          rethrow;
        } catch (_) {
          if (_disposed || attempt + 1 == setupReadinessAttempts) rethrow;
          await Future<void>.delayed(setupRetryDelay);
          if (_disposed) return;
        }
      }
    } catch (_) {
      if (_disposed) return;
      setupError = setupPhase == ServiceSetupPhase.checking
          ? (language == 'fa'
                ? 'عملیات نصب یا راه‌اندازی پایان یافت، اما سرویس هنوز پاسخ نمی‌دهد. دوباره بررسی کنید.'
                : 'Installation or startup finished, but the service is not responding yet. Try again.')
          : (language == 'fa'
                ? 'راه‌اندازی کامل نشد. درخواست دسترسی مدیر یا نصب را دوباره امتحان کنید.'
                : 'Setup did not complete. Try the administrator approval or installation again.');
      setupPhase = ServiceSetupPhase.failed;
      state = {};
    } finally {
      probe?.close();
      _setupClient = null;
      busy = false;
      if (!_disposed) notifyListeners();
    }
  }

  void acknowledgeSetupPassword() {
    setupPassword = null;
    setupPasswordAcknowledged = true;
    if (!_disposed) notifyListeners();
  }

  void _rememberSetupPassword(String result) {
    for (final line in result.split(RegExp(r'\r?\n'))) {
      for (final prefix in [
        'first-run panel password: ',
        'New Caspian panel password: ',
      ]) {
        if (line.startsWith(prefix) && line.length > prefix.length) {
          setupPassword = line.substring(prefix.length);
          setupPasswordAcknowledged = false;
        }
      }
    }
  }

  Future<void> serviceAction(String action) async {
    if (!desktop || busy || _disposed) return;
    busy = true;
    serviceMessage = null;
    notifyListeners();
    try {
      serviceMessage = await _lifecycle.run(action);
      if (action == 'reset-password') {
        // Successful reset invalidates the previous credential. Windows shows
        // the replacement in its elevated dialog instead of returning it here.
        setupPassword = null;
        setupPasswordAcknowledged = false;
        _rememberSetupPassword(serviceMessage!);
      }
    } catch (_) {
      serviceMessage = language == 'fa'
          ? 'عملیات سرویس انجام نشد. ممکن است دسترسی مدیر رد شده باشد.'
          : 'The service action did not complete. Administrator access may have been declined.';
    } finally {
      busy = false;
      if (!_disposed) {
        notifyListeners();
        await refresh();
      }
    }
  }

  @override
  void dispose() {
    _disposed = true;
    _timer?.cancel();
    _setupClient?.close();
    _client.close();
    setupPassword = null;
    super.dispose();
  }
}
