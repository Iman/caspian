// SPDX-License-Identifier: AGPL-3.0-or-later
import 'lifecycle.dart';

class PlatformLifecycle implements DesktopLifecycle {
  @override
  Future<String> run(String action) =>
      Future.error(UnsupportedError('Desktop only'));
}
