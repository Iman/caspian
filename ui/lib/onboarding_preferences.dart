// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:shared_preferences/shared_preferences.dart';

class OnboardingPreferences {
  OnboardingPreferences({
    Future<bool?> Function(String)? read,
    Future<void> Function(String, bool)? write,
  }) : _read = read ?? ((key) => SharedPreferencesAsync().getBool(key)),
       _write =
           write ??
           ((key, value) => SharedPreferencesAsync().setBool(key, value));

  final Future<bool?> Function(String) _read;
  final Future<void> Function(String, bool) _write;

  static const _welcome = 'caspian.onboarding.welcome.v1';
  static const _checklist = 'caspian.onboarding.checklist.v1';

  Future<bool> welcomeCompleted() => _completed(_welcome);
  Future<bool> checklistCompleted() => _completed(_checklist);
  Future<bool> completeWelcome() => _complete(_welcome);
  Future<bool> completeChecklist() => _complete(_checklist);

  Future<bool> _completed(String key) async {
    try {
      return await _read(key) ?? false;
    } catch (_) {
      return false;
    }
  }

  Future<bool> _complete(String key) async {
    try {
      await _write(key, true);
      return true;
    } catch (_) {
      return false;
    }
  }
}
