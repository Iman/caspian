import 'dart:io';
import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:caspian/lifecycle_io.dart';

void main() {
  test('Linux install returns only the exact generated password line', () async {
    final lifecycle = PlatformLifecycle(
      operatingSystem: 'linux',
      executable: '/opt/caspian/caspian_ui',
      execute: (_, _) async => ProcessResult(
        1,
        0,
        'Panel: http://192.0.2.1:8088/\nPassword: abcde-fghij-kmnpq-rstuv\nprivate network details\nPassword: unchanged from the previous install\n',
        '',
      ),
    );
    expect(
      await lifecycle.run('install'),
      'first-run panel password: abcde-fghij-kmnpq-rstuv',
    );
    expect(await lifecycle.run('start'), 'The service action completed.');
  });
  test(
    'macOS elevation quotes bundle paths and reveals only issued password',
    () async {
      String command = '';
      List<String> arguments = [];
      final lifecycle = PlatformLifecycle(
        operatingSystem: 'macos',
        executable: "/Applications/Owner's Caspian.app/Contents/MacOS/Caspian",
        execute: (program, args) async {
          command = program;
          arguments = args;
          return ProcessResult(
            1,
            0,
            'private installation details\nfirst-run panel password: issued-password\n',
            'private error',
          );
        },
      );
      final result = await lifecycle.run('install');
      expect(command, '/usr/bin/osascript');
      expect(arguments.last, contains("Owner'\\''s Caspian.app"));
      expect(arguments.last, contains('CASPIAN_LOCAL_BINARY='));
      expect(result, 'first-run panel password: issued-password');
    },
  );

  test(
    'Linux service actions use a fixed privileged script and closed action',
    () async {
      final calls = <List<String>>[];
      final lifecycle = PlatformLifecycle(
        operatingSystem: 'linux',
        executable: '/opt/caspian/caspian_ui',
        execute: (program, args) async {
          calls.add([program, ...args]);
          return ProcessResult(1, 0, '', '');
        },
      );
      await lifecycle.run('restart');
      expect(calls.single, [
        'pkexec',
        '/bin/bash',
        '/opt/caspian/service-action.sh',
        'restart',
      ]);
      await expectLater(
        lifecycle.run('restart; arbitrary-command'),
        throwsArgumentError,
      );
      expect(calls.length, 1);
    },
  );

  test(
    'Windows passes fixed helper and action as separate arguments',
    () async {
      final calls = <List<String>>[];
      final lifecycle = PlatformLifecycle(
        operatingSystem: 'windows',
        executable: r'C:\Program Files\Caspian\caspian_ui.exe',
        windowsRoot: r'C:\Windows',
        execute: (program, args) async {
          calls.add([program, ...args]);
          return ProcessResult(1, 0, 'not a credential', '');
        },
      );
      final result = await lifecycle.run('reset-password');
      expect(
        calls.single.first,
        r'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe',
      );
      expect(calls.single, contains(r'C:\Program Files\Caspian\lifecycle.ps1'));
      expect(calls.single.last, 'reset-password');
      expect(result, isNot(contains('not a credential')));
    },
  );

  test('failed elevation never returns captured output or passwords', () async {
    final lifecycle = PlatformLifecycle(
      operatingSystem: 'linux',
      executable: '/opt/caspian/caspian_ui',
      execute: (_, _) async => ProcessResult(
        1,
        1,
        'New Caspian panel password: secret',
        'sensitive failure',
      ),
    );
    await expectLater(
      lifecycle.run('reset-password'),
      throwsA(
        isA<StateError>().having(
          (e) => e.toString(),
          'message',
          isNot(contains('secret')),
        ),
      ),
    );
  });

  test('macOS start and reset select only the bundled scripts', () async {
    final calls = <List<String>>[];
    final lifecycle = PlatformLifecycle(
      operatingSystem: 'macos',
      executable: '/Applications/Caspian.app/Contents/MacOS/Caspian',
      execute: (program, args) async {
        calls.add([program, ...args]);
        return ProcessResult(1, 0, '', '');
      },
    );
    await lifecycle.run('start');
    await lifecycle.run('reset-password');
    expect(
      calls[0].last,
      "/bin/bash '/Applications/Caspian.app/Contents/Resources/service-action.sh' start",
    );
    expect(
      calls[1].last,
      "/bin/bash '/Applications/Caspian.app/Contents/Resources/reset-password.sh'",
    );
  });

  test(
    'Linux install and password reset do not interpolate shell commands',
    () async {
      final calls = <List<String>>[];
      final lifecycle = PlatformLifecycle(
        operatingSystem: 'linux',
        executable: '/tmp/Extracted Caspian/caspian_ui',
        execute: (program, args) async {
          calls.add([program, ...args]);
          return ProcessResult(1, 0, '', '');
        },
      );
      await lifecycle.run('install');
      await lifecycle.run('reset-password');
      expect(calls[0], [
        'pkexec',
        '/bin/bash',
        '/tmp/Extracted Caspian/install-desktop.sh',
      ]);
      expect(calls[1], ['pkexec', '/usr/local/bin/caspian', 'reset-password']);
    },
  );

  test(
    'Windows installation uses the fixed helper and unsupported platforms refuse',
    () async {
      var calls = 0;
      Future<ProcessResult> execute(String _, List<String> arguments) async {
        calls++;
        expect(arguments, contains(r'C:\Caspian\lifecycle.ps1'));
        expect(arguments.last, 'install');
        return ProcessResult(1, 0, '', '');
      }

      final windows = PlatformLifecycle(
        operatingSystem: 'windows',
        executable: r'C:\Caspian\caspian_ui.exe',
        execute: execute,
      );
      await windows.run('install');
      final unsupported = PlatformLifecycle(
        operatingSystem: 'android',
        executable: '/app/caspian',
        execute: execute,
      );
      await expectLater(unsupported.run('start'), throwsUnsupportedError);
      expect(calls, 1);
    },
  );

  test('launch errors are sanitized and release the operation lock', () async {
    var calls = 0;
    final lifecycle = PlatformLifecycle(
      operatingSystem: 'linux',
      executable: '/opt/caspian/caspian_ui',
      execute: (program, args) async {
        calls++;
        if (calls == 1) {
          throw ProcessException(program, args, 'sensitive process failure');
        }
        return ProcessResult(1, 0, '', '');
      },
    );
    await expectLater(
      lifecycle.run('start'),
      throwsA(
        isA<StateError>().having(
          (e) => e.toString(),
          'sanitized',
          isNot(contains('sensitive')),
        ),
      ),
    );
    expect(await lifecycle.run('start'), 'The service action completed.');
  });

  test('an active elevated operation prevents overlapping actions', () async {
    final completion = Completer<ProcessResult>();
    final lifecycle = PlatformLifecycle(
      operatingSystem: 'linux',
      executable: '/opt/caspian/caspian_ui',
      execute: (_, _) => completion.future,
    );
    final first = lifecycle.run('start');
    await expectLater(lifecycle.run('stop'), throwsStateError);
    completion.complete(ProcessResult(1, 0, '', ''));
    expect(await first, 'The service action completed.');
  });
}
