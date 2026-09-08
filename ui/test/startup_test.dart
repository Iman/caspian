// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:caspian/main.dart' as app;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'application starts without service and disposes polling on close',
    (tester) async {
      app.main();
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));
      expect(find.byType(app.CaspianApplication), findsOneWidget);
      // The widget-test HTTP override supplies an unavailable backend. Startup
      // must keep the application mounted instead of throwing or exiting.
      expect(tester.takeException(), isNull);
      await tester.pump(const Duration(seconds: 5));
      await tester.pumpWidget(const SizedBox());
      await tester.pump();
    },
  );
}
