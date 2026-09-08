// SPDX-License-Identifier: AGPL-3.0-or-later
import 'package:flutter/material.dart';

/// Colour and geometry tokens from internal/panel/assets/panel.css.
abstract final class CaspianStyle {
  static const teal = Color(0xFF097C87);
  static const cyan = Color(0xFF23CED9);
  static const sage = Color(0xFFA1CCA6);
  static const coral = Color(0xFFFCA47C);
  static const yellow = Color(0xFFF9D779);
  static const ground = Color(0xFFE6F2F3);
  static const edge = Color(0xFFC2DEE1);
  static const ink = Color(0xFF05444A);
  static const quiet = Color(0xFF075D65);
  static const surface = Colors.white;
  static const railWidth = 268.0;
  static const contentMax = 1180.0;

  static ThemeData get theme => ThemeData(
    useMaterial3: true,
    scaffoldBackgroundColor: ground,
    colorScheme: const ColorScheme.light(
      primary: teal,
      onPrimary: surface,
      secondary: sage,
      onSecondary: ink,
      surface: surface,
      onSurface: ink,
      error: ink,
      onError: coral,
      outline: teal,
    ),
    fontFamily: 'Vazirmatn',
    fontFamilyFallback: const ['CaspianSans'],
    textTheme: const TextTheme(
      bodyMedium: TextStyle(fontSize: 16, height: 1.75, color: ink),
      bodyLarge: TextStyle(fontSize: 16, height: 1.75, color: ink),
      titleLarge: TextStyle(
        fontSize: 20,
        height: 1.35,
        fontWeight: FontWeight.w700,
        color: ink,
      ),
      titleMedium: TextStyle(
        fontSize: 17,
        height: 1.35,
        fontWeight: FontWeight.w700,
        color: ink,
      ),
    ),
    dividerColor: edge,
    dividerTheme: const DividerThemeData(color: edge, thickness: 1),
    cardTheme: CardThemeData(
      elevation: 0,
      color: surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: edge),
      ),
      margin: EdgeInsets.zero,
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: surface,
      alignLabelWithHint: true,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(9),
        borderSide: const BorderSide(color: teal),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(9),
        borderSide: const BorderSide(color: teal),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(9),
        borderSide: const BorderSide(color: ink, width: 2),
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: teal,
        foregroundColor: surface,
        elevation: 0,
        padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 18),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(9)),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: teal,
        backgroundColor: surface,
        side: const BorderSide(color: teal),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
      ),
    ),
    expansionTileTheme: const ExpansionTileThemeData(
      tilePadding: EdgeInsets.zero,
      childrenPadding: EdgeInsets.only(top: 12),
      iconColor: teal,
      textColor: teal,
      collapsedIconColor: teal,
      collapsedTextColor: teal,
      shape: Border(),
      collapsedShape: Border(),
    ),
  );
}
