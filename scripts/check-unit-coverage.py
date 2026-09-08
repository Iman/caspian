#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Validate fresh test evidence and enforce measured native UI/panel coverage."""
import argparse
import json
from pathlib import Path
import sys


def fresh(path, since):
    if not path.is_file() or path.stat().st_size == 0:
        raise ValueError(f'missing or empty evidence: {path}')
    if path.stat().st_mtime < since:
        raise ValueError(f'stale evidence: {path}')


def enforce(label, covered, total, minimum):
    if total <= 0:
        raise ValueError(f'{label}: no executable statements or lines measured')
    print(f'{label}: {covered}/{total} = {100 * covered / total:.2f}% (minimum {minimum:g}%)')
    # Compare integer counts, not the rounded display percentage.
    if covered * 100 < minimum * total:
        raise ValueError(f'{label}: below coverage minimum')


def read_lcov(path):
    sources = {}
    source = None
    for line in path.read_text().splitlines():
        if line.startswith('SF:'):
            if source is not None: raise ValueError('LCOV record is incomplete')
            source = line[3:]
            if not source: raise ValueError('LCOV source path is empty')
            sources.setdefault(source, {})
        elif line.startswith('DA:'):
            if source is None: raise ValueError('LCOV line has no source')
            values = line[3:].split(',')
            number, count = int(values[0]), int(values[1])
            if number <= 0 or count < 0: raise ValueError('invalid LCOV line/count')
            sources[source][number] = max(count, sources[source].get(number, 0))
        elif line == 'end_of_record':
            source = None
    if source is not None: raise ValueError('LCOV record is incomplete')
    if not sources or not any(sources.values()):
        raise ValueError('LCOV contains no measured lines')
    return {name: (sum(count > 0 for count in lines.values()), len(lines))
            for name, lines in sources.items()}


def read_go(path):
    lines = path.read_text().splitlines()
    if not lines or lines[0] not in ('mode: set', 'mode: count', 'mode: atomic'):
        raise ValueError('missing Go coverprofile mode')
    blocks = {}
    for line in lines[1:]:
        position, statements, hits = line.rsplit(' ', 2)
        statements, hits = int(statements), int(hits)
        if statements < 0 or hits < 0: raise ValueError('negative Go coverage count')
        previous = blocks.get(position)
        if previous and previous[0] != statements:
            raise ValueError('inconsistent duplicate Go coverage block')
        blocks[position] = (statements, max(hits, previous[1] if previous else 0))
    total = [0, 0]
    panel = [0, 0]
    for position, (statements, hits) in blocks.items():
        groups = [total]
        filename = position.rsplit(':', 1)[0]
        if filename.rsplit('/', 1)[0].endswith('/internal/panel'):
            groups.append(panel)
        for group in groups:
            group[0] += statements if hits > 0 else 0
            group[1] += statements
    if not total[1]: raise ValueError('Go profile contains no measured statements')
    return tuple(total), tuple(panel)


def flutter_counts(rows):
    completed = [r for r in rows if r.get('type') == 'done']
    tests = [r for r in rows if r.get('type') == 'testDone' and not r.get('hidden', False)]
    if not completed or completed[-1].get('success') is not True:
        raise ValueError('Flutter run did not finish successfully')
    if any(r.get('result') != 'success' for r in tests if not r.get('skipped')):
        raise ValueError('Flutter test failure')
    passed = len({r['testID'] for r in tests if not r.get('skipped')})
    skipped = sum(bool(r.get('skipped')) for r in tests)
    if passed == 0: raise ValueError('Flutter executed zero successful tests')
    return passed, skipped


def go_counts(rows):
    if any(r.get('Action') in ('fail', 'build-fail') for r in rows):
        raise ValueError('Go test or build failure')
    runs = {(r.get('Package'), r['Test']) for r in rows if r.get('Action') == 'run' and r.get('Test')}
    passed = {(r.get('Package'), r['Test']) for r in rows if r.get('Action') == 'pass' and r.get('Test')}
    skipped = {(r.get('Package'), r['Test']) for r in rows if r.get('Action') == 'skip' and r.get('Test')}
    finished_packages = {r.get('Package') for r in rows if r.get('Action') in ('pass', 'skip') and not r.get('Test')}
    test_packages = {package for package, _ in runs}
    if not test_packages <= finished_packages:
        raise ValueError('Go package run did not finish')
    if not passed or not passed <= runs or not runs <= passed | skipped:
        raise ValueError('Go executed no successful tests or has incomplete results')
    return len(passed), len(skipped)


def read_events(path):
    rows = [json.loads(line) for line in path.read_text().splitlines() if line.strip()]
    if not rows or any(not isinstance(row, dict) for row in rows):
        raise ValueError(f'invalid test events: {path}')
    return rows


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('lcov', 'flutter-results', 'go-profile', 'go-results'):
        parser.add_argument('--' + name, required=True, type=Path)
    parser.add_argument('--since', required=True, type=float, help='Unix time recorded before this test run')
    parser.add_argument('--minimum', type=float, default=80)
    parser.add_argument('--dart-source-root', type=Path, default=Path('ui/lib'))
    args = parser.parse_args(argv)
    try:
        if not 0 <= args.minimum <= 100: raise ValueError('minimum must be between 0 and 100')
        for path in (args.lcov, args.flutter_results, args.go_profile, args.go_results): fresh(path, args.since)
        flutter_passed, flutter_skipped = flutter_counts(read_events(args.flutter_results))
        go_events = read_events(args.go_results)
        go_passed, go_skipped = go_counts(go_events)
        # The gated package must itself have executed, not merely occur in a
        # profile generated by a different subset of packages.
        panel_passed, panel_skipped = go_counts([r for r in go_events if r.get('Package', '').endswith('/internal/panel')])
        print(f'Flutter tests: {flutter_passed} passed, {flutter_skipped} skipped')
        print(f'Go tests/subtests: {go_passed} passed, {go_skipped} skipped')
        print(f'Go panel tests/subtests: {panel_passed} passed, {panel_skipped} skipped')
        sources = read_lcov(args.lcov)
        names = {name.replace('\\', '/').split('lib/', 1)[-1] for name in sources}
        missing = []
        for source in sorted(args.dart_source_root.rglob('*.dart')):
            relative = source.relative_to(args.dart_source_root).as_posix()
            if relative in names: continue
            if relative in ('transport_web.dart', 'lifecycle_web.dart'):
                print(f'UNKNOWN native unit coverage: {relative} is compiled only for web; browser integration is separate evidence')
            elif relative == 'lifecycle.dart':
                print('Coverage scope: lifecycle.dart declares the conditional factory; executable implementations are gated through LCOV')
            else:
                missing.append(relative)
        if missing: raise ValueError('native Dart source absent from LCOV: ' + ', '.join(missing))
        covered, total = map(sum, zip(*sources.values()))
        enforce('Flutter native Dart lines (all LCOV records, no record exclusions)', covered, total, args.minimum)
        overall, panel = read_go(args.go_profile)
        enforce('Go complete profile statements', *overall, args.minimum)
        enforce('Go panel statements', *panel, args.minimum)
        print('Existing scripts/gate.sh package floors remain independently required.')
    except (ValueError, OSError, KeyError, IndexError) as error:
        print(f'FAIL: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
