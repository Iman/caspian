import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('coverage_gate', Path(__file__).with_name('check-unit-coverage.py'))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


class CoverageGateTests(unittest.TestCase):
    def test_good_panel_coverage_cannot_hide_low_overall_go_coverage(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'app.dart').write_text('void main() {}')
            (root / 'flutter.lcov').write_text('SF:lib/app.dart\nDA:1,1\nend_of_record\n')
            flutter = [{'type': 'testDone', 'testID': 1, 'result': 'success'}, {'type': 'done', 'success': True}]
            go = [{'Action': 'run', 'Package': 'example/internal/panel', 'Test': 'TestA'}, {'Action': 'pass', 'Package': 'example/internal/panel', 'Test': 'TestA'}, {'Action': 'pass', 'Package': 'example/internal/panel'}]
            for name, rows in [('flutter.json', flutter), ('go.json', go)]:
                (root / name).write_text('\n'.join(map(json.dumps, rows)))
            args = ['--lcov', str(root / 'flutter.lcov'), '--flutter-results', str(root / 'flutter.json'), '--go-profile', str(root / 'go.cover'), '--go-results', str(root / 'go.json'), '--since', '0', '--dart-source-root', str(root)]
            panel = 'mode: set\nexample/internal/panel/a.go:1.1,2.1 4 1\nexample/internal/panel/a.go:3.1,4.1 1 0\n'
            (root / 'go.cover').write_text(panel)
            self.assertEqual(gate.main(args), 0)
            (root / 'go.cover').write_text(panel + 'example/internal/other/a.go:1.1,2.1 5 0\n')
            self.assertEqual(gate.main(args), 1)

    def test_exact_floor_passes_and_below_fails_without_rounding(self):
        gate.enforce('sample', 4, 5, 80)
        with self.assertRaises(ValueError):
            gate.enforce('sample', 7999, 10000, 80)

    def test_zero_statements_are_not_coverage(self):
        with self.assertRaises(ValueError):
            gate.enforce('sample', 0, 0, 80)

    def test_lcov_counts_distinct_lines_and_retains_uncovered_lines(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'coverage.info'
            path.write_text('SF:lib/app.dart\nDA:1,2\nDA:2,0\nend_of_record\nSF:lib/app.dart\nDA:1,1\nDA:2,0\nend_of_record\n')
            self.assertEqual(gate.read_lcov(path), {'lib/app.dart': (1, 2)})

    def test_go_profile_weights_statements_and_reports_panel_separately(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'go.cover'
            path.write_text('mode: set\nexample/internal/panel/a.go:1.1,2.1 4 1\nexample/internal/panel/a.go:3.1,4.1 1 0\nexample/internal/other/a.go:1.1,2.1 5 0\n')
            total, panel = gate.read_go(path)
            self.assertEqual(total, (4, 10))
            self.assertEqual(panel, (4, 5))

    def test_empty_and_malformed_profiles_fail(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'empty'
            path.write_text('')
            for reader in (gate.read_lcov, gate.read_go):
                with self.assertRaises(ValueError): reader(path)
            path.write_text('SF:lib/app.dart\nDA:1,1\n')
            with self.assertRaises(ValueError): gate.read_lcov(path)

    def test_flutter_requires_completed_nonzero_successful_tests(self):
        good = [{'type': 'testDone', 'testID': 1, 'result': 'success', 'skipped': False, 'hidden': False}, {'type': 'done', 'success': True}]
        self.assertEqual(gate.flutter_counts(good), (1, 0))
        for rows in ([], [{'type': 'done', 'success': True}], good[:-1], [*good[:-1], {'type': 'done', 'success': False}]):
            with self.assertRaises(ValueError): gate.flutter_counts(rows)

    def test_go_requires_executed_tests_and_rejects_package_failure(self):
        good = [{'Action': 'run', 'Package': 'sample', 'Test': 'TestA'}, {'Action': 'pass', 'Package': 'sample', 'Test': 'TestA'}, {'Action': 'pass', 'Package': 'sample'}]
        self.assertEqual(gate.go_counts(good), (1, 0))
        for rows in ([], [{'Action': 'pass', 'Package': 'sample'}], good[:-1], good + [{'Action': 'fail', 'Package': 'sample'}]):
            with self.assertRaises(ValueError): gate.go_counts(rows)

    def test_stale_evidence_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'result'
            path.write_text('fresh')
            gate.fresh(path, path.stat().st_mtime - 1)
            with self.assertRaises(ValueError): gate.fresh(path, path.stat().st_mtime + 1)


if __name__ == '__main__':
    unittest.main()
