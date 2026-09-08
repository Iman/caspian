// SPDX-License-Identifier: AGPL-3.0-or-later
import 'dart:io';
import 'lifecycle.dart';

typedef ProcessExecutor = Future<ProcessResult> Function(String, List<String>);

class PlatformLifecycle implements DesktopLifecycle {
  PlatformLifecycle({
    String? operatingSystem,
    String? executable,
    String? windowsRoot,
    ProcessExecutor? execute,
  }) : _platform = operatingSystem ?? Platform.operatingSystem,
       _executable = executable ?? Platform.resolvedExecutable,
       _windowsRoot =
           windowsRoot ?? Platform.environment['SystemRoot'] ?? r'C:\Windows',
       _execute =
           execute ?? ((program, arguments) => Process.run(program, arguments));

  final String _platform;
  final String _executable;
  final String _windowsRoot;
  final ProcessExecutor _execute;
  bool _busy = false;

  String _parent(String path) =>
      path.substring(0, path.lastIndexOf(_platform == 'windows' ? r'\' : '/'));
  static String _quote(String value) => "'${value.replaceAll("'", "'\\''")}'";

  @override
  Future<String> run(String action) async {
    if (!{
      'install',
      'start',
      'stop',
      'restart',
      'reset-password',
    }.contains(action)) {
      throw ArgumentError.value(action, 'action');
    }
    if (_busy) throw StateError('A service action is already active.');
    _busy = true;
    try {
      final directory = _parent(_executable);
      final String program;
      final List<String> arguments;
      switch (_platform) {
        case 'macos':
          final resources = '${_parent(directory)}/Resources';
          final String command;
          if (action == 'install') {
            command =
                'CASPIAN_LOCAL_BINARY=${_quote('$resources/caspian')} /bin/bash ${_quote('$resources/install-darwin.sh')}';
          } else if (action == 'reset-password') {
            command = '/bin/bash ${_quote('$resources/reset-password.sh')}';
          } else {
            command =
                '/bin/bash ${_quote('$resources/service-action.sh')} $action';
          }
          program = '/usr/bin/osascript';
          arguments = [
            '-e',
            'on run argv\nreturn do shell script (item 1 of argv) with administrator privileges\nend run',
            command,
          ];
        case 'linux':
          program = 'pkexec';
          arguments = switch (action) {
            'install' => ['/bin/bash', '$directory/install-desktop.sh'],
            'reset-password' => ['/usr/local/bin/caspian', 'reset-password'],
            _ => ['/bin/bash', '$directory/service-action.sh', action],
          };
        case 'windows':
          program =
              '$_windowsRoot\\System32\\WindowsPowerShell\\v1.0\\powershell.exe';
          arguments = [
            '-NoProfile',
            '-NonInteractive',
            '-ExecutionPolicy',
            'Bypass',
            '-File',
            '$directory\\lifecycle.ps1',
            '-Action',
            action,
          ];
        default:
          throw UnsupportedError(
            'Desktop service actions are unavailable on this platform.',
          );
      }
      // Do not time out and release the operation lock: terminating the launcher
      // cannot establish whether its elevated child has finished changing services.
      final result = await _execute(program, arguments);
      if (result.exitCode != 0) {
        throw StateError('The service action did not complete.');
      }
      final credentials = result.stdout
          .toString()
          .split(RegExp(r'\r?\n'))
          .map((line) {
            // install.sh generates four groups of five characters from this
            // alphabet. Do not mistake its upgrade notice or network output
            // for a newly issued credential.
            if (_platform == 'linux' && action == 'install') {
              final issued = RegExp(
                r'^Password: ([abcdefghijkmnpqrstuvwxyz23456789]{5}(?:-[abcdefghijkmnpqrstuvwxyz23456789]{5}){3})$',
              ).firstMatch(line);
              if (issued != null) {
                return 'first-run panel password: ${issued.group(1)}';
              }
            }
            return line;
          })
          .where(
            (line) =>
                line.startsWith('first-run panel password: ') ||
                line.startsWith('New Caspian panel password: '),
          )
          .join('\n');
      if (credentials.isNotEmpty) return credentials;
      return _platform == 'windows' && action == 'reset-password'
          ? 'The new password was displayed in the administrator window.'
          : 'The service action completed.';
    } on ProcessException {
      throw StateError('The service action could not start.');
    } finally {
      _busy = false;
    }
  }
}
