// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:caspian/onboarding_preferences.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'unavailable platform preferences do not prevent application startup',
    () async {
      final preferences = OnboardingPreferences();
      expect(await preferences.welcomeCompleted(), isFalse);
      expect(await preferences.checklistCompleted(), isFalse);
      expect(await preferences.completeWelcome(), isFalse);
      expect(await preferences.completeChecklist(), isFalse);
    },
  );

  test(
    'first run is incomplete and acknowledgements survive a new instance',
    () async {
      final values = <String, bool>{};
      OnboardingPreferences preferences() => OnboardingPreferences(
        read: (key) async => values[key],
        write: (key, value) async {
          values[key] = value;
        },
      );
      final first = preferences();
      expect(await first.welcomeCompleted(), isFalse);
      expect(await first.checklistCompleted(), isFalse);
      expect(await first.completeWelcome(), isTrue);
      expect(await preferences().welcomeCompleted(), isTrue);
      expect(await preferences().checklistCompleted(), isFalse);
      expect(await first.completeChecklist(), isTrue);
      expect(await preferences().checklistCompleted(), isTrue);
      expect(values, {
        'caspian.onboarding.welcome.v1': true,
        'caspian.onboarding.checklist.v1': true,
      });
    },
  );

  test(
    'unreadable preferences show guidance without blocking startup',
    () async {
      final preferences = OnboardingPreferences(
        read: (_) async => throw StateError('Synthetic storage refusal'),
        write: (_, _) async {},
      );
      expect(await preferences.welcomeCompleted(), isFalse);
      expect(await preferences.checklistCompleted(), isFalse);
    },
  );

  test('failed persistence does not report a saved acknowledgement', () async {
    final preferences = OnboardingPreferences(
      read: (_) async => null,
      write: (_, _) async => throw StateError('Synthetic storage refusal'),
    );
    expect(await preferences.completeWelcome(), isFalse);
    expect(await preferences.completeChecklist(), isFalse);
    expect(await preferences.welcomeCompleted(), isFalse);
    expect(await preferences.checklistCompleted(), isFalse);
  });
}
