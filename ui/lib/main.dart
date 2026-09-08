// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'app.dart';
import 'controller.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const CaspianApplication());
}

class CaspianApplication extends StatefulWidget {
  const CaspianApplication({super.key});
  @override
  State<CaspianApplication> createState() => _CaspianApplicationState();
}

class _CaspianApplicationState extends State<CaspianApplication> {
  late final CaspianController controller;
  late final SemanticsHandle semantics;
  @override
  void initState() {
    super.initState();
    semantics = SemanticsBinding.instance.ensureSemantics();
    controller = CaspianController()..start();
  }

  @override
  Widget build(BuildContext context) => CaspianApp(controller: controller);
  @override
  void dispose() {
    controller.dispose();
    semantics.dispose();
    super.dispose();
  }
}
