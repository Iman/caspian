#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Regression tests for wiki navigation and translation checks."""

import importlib.util
from pathlib import Path
import shutil
import tempfile
import unittest

import wiki_navigation as nav

spec = importlib.util.spec_from_file_location("check_wiki", Path(__file__).with_name("check-wiki.py"))
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)
SOURCE = Path(__file__).resolve().parents[1] / "docs/wiki"


class WikiNavigationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "wiki"
        shutil.copytree(SOURCE, self.root)

    def test_current_wiki_and_all_topics_are_reachable(self):
        self.assertEqual(checker.check(self.root)[1], [])
        for lang in nav.LANGUAGES:
            home = (self.root / (nav.slug("Home", lang) + ".md")).read_text(encoding="utf-8")
            for topic in nav.TOPICS:
                self.assertIn(nav.BASE + nav.slug(topic, lang) + ")", home)

    def test_wrong_language_in_navigation_is_rejected(self):
        page = self.root / "Getting-Started.fa.md"
        text = page.read_text(encoding="utf-8").replace("wiki/Home.fa)", "wiki/Home)")
        page.write_text(text, encoding="utf-8")
        self.assertTrue(any("incorrect localized navigation" in e for e in checker.check(self.root)[1]))

    def test_translated_sidebar_is_rejected(self):
        (self.root / "_Sidebar.fa.md").write_text("obsolete", encoding="utf-8")
        self.assertTrue(any("one shared" in e for e in checker.check(self.root)[1]))

    def test_missing_target_with_digits_query_and_fragment_is_rejected(self):
        with (self.root / "_Sidebar.md").open("a", encoding="utf-8") as page:
            page.write(f"\n[Broken]({nav.BASE}Missing-123?view=1#setup)\n")
        self.assertTrue(any("no page: Missing-123" in e for e in checker.check(self.root)[1]))

    def test_technical_translation_drift_is_rejected(self):
        with (self.root / "Getting-Started.fa.md").open("a", encoding="utf-8") as page:
            page.write("\n`changed-command`\n")
        self.assertTrue(any("inline code differ" in e for e in checker.check(self.root)[1]))

    def test_missing_rtl_wrapper_is_rejected(self):
        page = self.root / "Getting-Started.fa.md"
        page.write_text(page.read_text(encoding="utf-8").replace('<div dir="rtl" align="right">', '<div>'), encoding="utf-8")
        self.assertTrue(any("right-to-left content" in e for e in checker.check(self.root)[1]))


if __name__ == "__main__":
    unittest.main()
