// SPDX-License-Identifier: AGPL-3.0-or-later
import 'lifecycle_io.dart'
    if (dart.library.js_interop) 'lifecycle_web.dart'
    as platform;

abstract class DesktopLifecycle {
  factory DesktopLifecycle() = platform.PlatformLifecycle;
  Future<String> run(String action);
}
