import importlib.machinery
import json
import importlib.util
import pathlib
import os
import re
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
loader = importlib.machinery.SourceFileLoader('ci_local_all', str(ROOT / 'scripts/ci-local-all'))
spec = importlib.util.spec_from_loader(loader.name, loader)
runner = importlib.util.module_from_spec(spec)
loader.exec_module(runner)


class LocalAllTest(unittest.TestCase):
    def test_required_mapping_tracks_workflow(self):
        required = runner.required_jobs((ROOT / '.github/workflows/ci.yml').read_text())
        self.assertEqual(required, set(runner.MAPPING))
        with self.assertRaisesRegex(ValueError, 'unmapped'):
            runner.check_mapping(required | {'new-required-job'})

    def test_parse_needs_fails_closed(self):
        with self.assertRaises(ValueError):
            runner.required_jobs('jobs:\n  required-ci:\n    needs: expression\n')

    def test_skip_is_not_success(self):
        self.assertNotEqual(runner.exit_status(['PASS', 'SKIP-ENV']), 0)
        self.assertNotEqual(runner.exit_status(['PASS', 'FAIL']), 0)
        self.assertEqual(runner.exit_status(['PASS', 'PASS']), 0)

    def test_first_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            log = pathlib.Path(directory) / 'lane.log'
            log.write_text('starting\n--- FAIL: TestA\nother failure\n')
            self.assertIn('FAIL: TestA', runner.first_failure(log))

    def test_missing_prerequisite_is_skip_without_running_command(self):
        with tempfile.TemporaryDirectory() as directory:
            r = runner.Runner(pathlib.Path(directory), 1)
            with mock.patch.object(runner.subprocess, 'run', side_effect=AssertionError('must not run')):
                r.command('missing', ['unused'], prerequisites=[(False, 'Node 22')])
            self.assertEqual(r.results['missing'][0], 'SKIP-ENV')
            self.assertIn('Node 22', r.results['missing'][1].read_text())

    def test_failed_dependency_is_failure_not_environment_skip(self):
        with tempfile.TemporaryDirectory() as directory:
            r = runner.Runner(pathlib.Path(directory), 1)
            r.command('dependent', ['unused'], prerequisites=[(False, 'npm ci failed', 'FAIL')])
            self.assertEqual(r.results['dependent'][0], 'FAIL')

    def test_failed_command_is_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            r = runner.Runner(pathlib.Path(directory), 1)
            r.command('bad', ['python3', '-c', 'import sys; print("error: sample"); sys.exit(7)'])
            self.assertEqual(r.results['bad'][0], 'FAIL')
            self.assertIn('error: sample', runner.first_failure(r.results['bad'][1]))

    def capture_plan(self, root):
        r = runner.Runner(root, 1)
        r.go = r.node = True
        r.browser_available = lambda package: True
        calls = {}
        def command(name, cmd, **kwargs):
            calls[name] = (cmd, kwargs)
            log = root / (name + '.log')
            log.write_text('fake pass\n')
            r.results[name] = ('PASS', log)
        def shell(name, script, **kwargs):
            command(name, script, **kwargs)
        r.command, r.shell = command, shell
        r.run()
        return calls

    def test_package_commands_match_workflow_jobs(self):
        with tempfile.TemporaryDirectory() as directory:
            calls = self.capture_plan(pathlib.Path(directory))
        workflow = (ROOT / '.github/workflows/ci.yml').read_text()
        for job, workflow_job, lane in [('server-and-playground', 'server-and-playground', 'server-and-playground'), ('test-guard', 'test', 'repoguard'), ('test-remaining-go', 'test', 'remaining-go')]:
            match = re.search(r'(?ms)^  ' + re.escape(workflow_job) + r':\n(.*?)(?=^  [a-z0-9-]+:|\Z)', workflow)
            self.assertIsNotNone(match, workflow_job)
            self.assertIn('scripts/ci-go-test.sh lane ' + lane, match.group(1))
            self.assertEqual(calls[job][0], ['scripts/ci-go-test.sh', 'lane', lane])

    def test_remaining_go_command_routes_match_workflow(self):
        with tempfile.TemporaryDirectory() as directory:
            calls = self.capture_plan(pathlib.Path(directory))
        workflow = (ROOT / '.github/workflows/ci.yml').read_text()
        for local, job, argument in [
            ('vet', 'vet', 'local-release-lane vet'),
            ('node-integration', 'node-integration', 'node-integration'),
            ('sema-0', 'sema', 'sema-shard'),
            ('gladecli-0', 'gladecli', 'gladecli-shard'),
            ('vm-0', 'vm', 'vm-shard'),
            ('lwc-compile-0', 'lwc-vf', 'lwc-compile-shard'),
            ('visualforce-0', 'lwc-vf', 'visualforce-shard'),
        ]:
            block = re.search(r'(?ms)^  ' + re.escape(job) + r':\n(.*?)(?=^  [a-z0-9-]+:|\Z)', workflow)
            self.assertIsNotNone(block, job)
            self.assertIn('scripts/ci-go-test.sh ' + argument, block.group(1))
            self.assertEqual(calls[local][0][:len(argument.split()) + 1], ['scripts/ci-go-test.sh', *argument.split()])
        self.assertEqual(calls['apextest-dispatch'][0][0], 'scripts/apextest-shards.sh')
        helper = (ROOT / 'scripts/apextest-shards.sh').read_text()
        apex = re.search(r'(?ms)^  apextest:\n(.*?)(?=^  [a-z0-9-]+:|\Z)', workflow).group(1)
        self.assertIn('ci-go-test.sh apex-shard', apex)
        self.assertIn('"${repo_root}/scripts/ci-go-test.sh" apex-shard', helper)

    def test_workflow_lint_runs_gofmt(self):
        with tempfile.TemporaryDirectory() as directory:
            calls = self.capture_plan(pathlib.Path(directory))
        workflow = (ROOT / '.github/workflows/ci.yml').read_text()
        block = re.search(r'(?ms)^  workflow-lint:\n(.*?)(?=^  [a-z0-9-]+:|\Z)', workflow).group(1)
        self.assertIn('scripts/ci-gofmt.sh', block)
        self.assertIn('scripts/ci-gofmt.sh', ' '.join(calls['workflow-lint'][0]))

    def test_cancel_shard_pool_stops_active_and_queued_work(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'scripts').mkdir()
            worker = root / 'scripts/ci-go-test.sh'
            worker.write_text('#!/bin/bash\nsleep 60 &\necho "$!" > "$MARKER.child"\necho "$*" >> "$MARKER"\nwait\n')
            worker.chmod(0o755)
            harness = root / 'harness.py'
            harness.write_text('''import importlib.machinery, importlib.util, pathlib, sys
loader = importlib.machinery.SourceFileLoader('runner', sys.argv[1])
spec = importlib.util.spec_from_loader(loader.name, loader)
m = importlib.util.module_from_spec(spec); loader.exec_module(m)
m.ROOT = pathlib.Path(sys.argv[2])
r = m.Runner(m.ROOT, 1); r.go = r.node = True
r.browser_available = lambda package: True
real = r.command
def command(name, cmd, **kw):
    if name.startswith('sema-'):
        return real(name, cmd, **kw)
    log = r.root / (name + '.log'); log.write_text('pass')
    r.results[name] = ('PASS', log)
r.command = command
r.run()
''')
            env = {**os.environ, 'MARKER': str(root / 'calls')}
            proc = subprocess.Popen([sys.executable, str(harness), str(ROOT / 'scripts/ci-local-all'), str(root)], env=env, start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                self.wait_for(root / 'calls')
                proc.send_signal(signal.SIGINT)
                proc.wait(timeout=5)
                self.assertNotEqual(proc.returncode, 0)
                self.assertEqual((root / 'calls').read_text().splitlines(), ['sema-shard 0'])
                self.assert_process_stopped(root / 'calls.child')
            finally:
                try: os.killpg(proc.pid, signal.SIGKILL)
                except ProcessLookupError: pass
                if (root / 'calls.child').exists():
                    try: os.kill(int((root / 'calls.child').read_text()), signal.SIGKILL)
                    except ProcessLookupError: pass
                proc.wait()

    @staticmethod
    def wait_for(path):
        deadline = time.monotonic() + 10
        while not path.exists():
            if time.monotonic() > deadline:
                raise AssertionError(f'fixture did not start: {path}')
            time.sleep(0.05)

    def assert_process_stopped(self, path):
        self.wait_for(path)
        pid = int(path.read_text())
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            state = subprocess.run(['ps', '-o', 'stat=', '-p', str(pid)], capture_output=True, text=True).stdout.strip()
            if not state or state.startswith('Z'):
                return
            time.sleep(0.05)
        self.fail(f'child {pid} survived cancellation')

    def test_apex_helper_term_stops_active_and_queued_shards(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'scripts').mkdir()
            shutil.copy(ROOT / 'scripts/apextest-shards.sh', root / 'scripts/apextest-shards.sh')
            worker = root / 'scripts/ci-go-test.sh'
            worker.write_text('#!/bin/bash\nsleep 60 &\necho "$!" > "$MARKER.child"\necho "$*" >> "$MARKER"\nwait\n')
            worker.chmod(0o755)
            env = {**os.environ, 'MARKER': str(root / 'calls'), 'CI_APEXTEST_SHARDS': '12', 'GLADE_APEXTEST_SHARD_JOBS': '1'}
            proc = subprocess.Popen(['bash', str(root / 'scripts/apextest-shards.sh'), str(root / 'evidence')], env=env, start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                self.wait_for(root / 'calls')
                proc.terminate()
                proc.wait(timeout=5)
                self.assertNotEqual(proc.returncode, 0)
                self.assertEqual((root / 'calls').read_text().splitlines(), ['apex-shard 0'])
                self.assert_process_stopped(root / 'calls.child')
            finally:
                try: os.killpg(proc.pid, signal.SIGKILL)
                except ProcessLookupError: pass
                if (root / 'calls.child').exists():
                    try: os.kill(int((root / 'calls.child').read_text()), signal.SIGKILL)
                    except ProcessLookupError: pass
                proc.wait()

    def test_apex_helper_timeout_stops_foreground_descendants(self):
        if not (shutil.which('gtimeout') or shutil.which('timeout')):
            self.skipTest('timeout executable is not installed')
        for count in (2, 6, 12):
            with self.subTest(shards=count), tempfile.TemporaryDirectory() as directory:
                root = pathlib.Path(directory)
                (root / 'scripts').mkdir()
                shutil.copy(ROOT / 'scripts/apextest-shards.sh', root / 'scripts/apextest-shards.sh')
                worker = root / 'scripts/ci-go-test.sh'
                worker.write_text('#!/bin/bash\nif [[ "$1" == apex-shard && "$2" == 0 ]]; then\n  bash -c \'echo "$$" > "$MARKER"; exec sleep 60\'\nfi\n')
                worker.chmod(0o755)
                env = {**os.environ, 'MARKER': str(root / 'child'), 'CI_APEXTEST_SHARDS': str(count), 'GLADE_APEXTEST_SHARD_JOBS': '1', 'GLADE_APEXTEST_SHARD_TIMEOUT_SECONDS': '1'}
                proc = subprocess.Popen(['bash', str(root / 'scripts/apextest-shards.sh'), str(root / 'evidence')], env=env, start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                try:
                    self.wait_for(root / 'child')
                    proc.wait(timeout=10)
                    self.assertNotEqual(proc.returncode, 0)
                    self.assert_process_stopped(root / 'child')
                finally:
                    try: os.killpg(proc.pid, signal.SIGKILL)
                    except ProcessLookupError: pass
                    if (root / 'child').exists():
                        try: os.kill(int((root / 'child').read_text()), signal.SIGKILL)
                        except ProcessLookupError: pass
                    proc.wait()

    def test_cancellation_kills_term_resistant_child_after_leader_exit(self):
        child = "import os,signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); open(os.environ['MARKER'],'w').write(str(os.getpid())); time.sleep(60)"
        for route in ('runner', 'helper-term', 'helper-timeout'):
            if route == 'helper-timeout' and not (shutil.which('gtimeout') or shutil.which('timeout')):
                continue
            with self.subTest(route=route), tempfile.TemporaryDirectory() as directory:
                root = pathlib.Path(directory)
                (root / 'scripts').mkdir()
                worker = root / 'scripts/ci-go-test.sh'
                worker.write_text('#!/bin/bash\nif [[ "$1" == apex-shard && "$2" != 0 ]]; then exit 0; fi\n' + shlex.quote(sys.executable) + ' -c ' + shlex.quote(child) + ' &\nwait\n')
                worker.chmod(0o755)
                env = {**os.environ, 'MARKER': str(root / 'child'), 'CI_APEXTEST_SHARDS': '12', 'GLADE_APEXTEST_SHARD_JOBS': '1', 'GLADE_APEXTEST_SHARD_TIMEOUT_SECONDS': '1'}
                if route == 'runner':
                    harness = root / 'harness.py'
                    harness.write_text('''import importlib.machinery,importlib.util,pathlib,sys
loader=importlib.machinery.SourceFileLoader('runner',sys.argv[1]); spec=importlib.util.spec_from_loader(loader.name,loader)
m=importlib.util.module_from_spec(spec); loader.exec_module(m); m.ROOT=pathlib.Path(sys.argv[2])
r=m.Runner(m.ROOT,1)
try: r.command('test',['scripts/ci-go-test.sh'])
except KeyboardInterrupt: sys.exit(130)
''')
                    command = [sys.executable, str(harness), str(ROOT / 'scripts/ci-local-all'), str(root)]
                else:
                    shutil.copy(ROOT / 'scripts/apextest-shards.sh', root / 'scripts/apextest-shards.sh')
                    command = ['bash', str(root / 'scripts/apextest-shards.sh'), str(root / 'evidence')]
                proc = subprocess.Popen(command, env=env, start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                try:
                    self.wait_for(root / 'child')
                    if route != 'helper-timeout':
                        proc.send_signal(signal.SIGINT if route == 'runner' else signal.SIGTERM)
                    proc.wait(timeout=15)
                    self.assertNotEqual(proc.returncode, 0)
                    self.assert_process_stopped(root / 'child')
                finally:
                    try: os.killpg(proc.pid, signal.SIGKILL)
                    except ProcessLookupError: pass
                    if (root / 'child').exists():
                        try: os.kill(int((root / 'child').read_text()), signal.SIGKILL)
                        except ProcessLookupError: pass
                    proc.wait()

    def test_codeql_cancellation_reaps_separate_evaluator(self):
        for sig in (signal.SIGINT, signal.SIGTERM):
            with self.subTest(signal=sig), tempfile.TemporaryDirectory() as directory:
                root = pathlib.Path(directory)
                (root / 'scripts').mkdir()
                shutil.copy(ROOT / 'scripts/ci-codeql.py', root / 'scripts/ci-codeql.py')
                (root / '.github').mkdir()
                shutil.copy(ROOT / '.github/codeql-budget.json', root / '.github/codeql-budget.json')
                fake = root / 'codeql'
                fake.write_text("#!/usr/bin/env python3\nimport os,signal,subprocess,time\nif 'finalize' in os.sys.argv:\n signal.signal(signal.SIGTERM, signal.SIG_IGN)\n child=subprocess.Popen(['sleep','60'])\n open(os.environ['MARKER'], 'w').write(str(os.getpid()))\n open(os.environ['MARKER']+'.child', 'w').write(str(child.pid))\n time.sleep(60)\n")
                fake.chmod(0o755)
                harness = root / 'harness.py'
                harness.write_text("""import importlib.machinery,importlib.util,pathlib,signal,sys
loader=importlib.machinery.SourceFileLoader('runner',sys.argv[1]); spec=importlib.util.spec_from_loader(loader.name,loader)
m=importlib.util.module_from_spec(spec); loader.exec_module(m); m.ROOT=pathlib.Path(sys.argv[2])
r=m.Runner(m.ROOT,1)
def interrupt(signum,frame): raise KeyboardInterrupt
signal.signal(signal.SIGTERM,interrupt)
try:
 r.shell('codeql', 'python3 scripts/ci-codeql.py analyze --profile fast --codeql ./codeql --database db --output result.sarif')
except KeyboardInterrupt:
 r.cancel(); sys.exit(130)
""")
                # A GitHub pull_request runner exports RUNNER_TEMP and GITHUB_EVENT_NAME,
                # which send ci-codeql.py to compute local PR diff ranges in this
                # git-less fixture before the fake evaluator ever starts.
                env = {k: v for k, v in os.environ.items() if not k.startswith(("GITHUB_", "RUNNER_", "CODEQL_"))}
                env["MARKER"] = str(root / "evaluator")
                proc = subprocess.Popen([sys.executable, str(harness), str(ROOT / 'scripts/ci-local-all'), str(root)], env=env, start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                try:
                    self.wait_for(root / 'evaluator.child')
                    proc.send_signal(sig)
                    proc.wait(timeout=12)
                    self.assertNotEqual(proc.returncode, 0)
                    # Cancellation must not report completion while either
                    # evaluator process is still running.
                    for path in (root / 'evaluator', root / 'evaluator.child'):
                        pid = int(path.read_text())
                        state = subprocess.run(['ps', '-o', 'stat=', '-p', str(pid)], capture_output=True, text=True).stdout.strip()
                        self.assertTrue(not state or state.startswith('Z'), f'process {pid} still running when cancellation returned')
                finally:
                    try: os.killpg(proc.pid, signal.SIGKILL)
                    except ProcessLookupError: pass
                    for path in (root / 'evaluator', root / 'evaluator.child'):
                        if path.exists():
                            try: os.kill(int(path.read_text()), signal.SIGKILL)
                            except ProcessLookupError: pass
                    proc.wait()

    def test_apex_ci_complete_history_flag_route(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            calls = self.capture_plan(root)
            env = calls['apextest-dispatch'][1]['env']
            self.assertEqual(env.get('CI_APEXTEST_HISTORY_COMPLETE'), '1')
            self.assertEqual(env['CI_APEXTEST_SHARDS'], '16')

    def test_apex_ci_history_collects_deferred_union(self):
        package = 'github.com/glade-sh/glade/internal/apextest'
        names = ['TestA%02d' % i for i in range(15)] + ['TestRunDataWeaveTypedAdmitted']
        shards = [dict(index=i, tests=[name], estimatedDurationMillis=0, regex='^(?:' + name + ')$') for i, name in enumerate(names)]
        plan = dict(version=1, package=package, historyUsed=False, shards=shards)
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'scripts').mkdir()
            shutil.copy(ROOT / 'scripts/apextest-shards.sh', root / 'scripts/apextest-shards.sh')
            for i, name in enumerate(names):
                fixture = root / ('fixture-%d' % i)
                fixture.mkdir()
                (fixture / 'discovery.txt').write_text('\n'.join(names) + '\n')
                (fixture / 'plan.json').write_text(json.dumps(plan))
                (fixture / 'selected-shard.json').write_text(json.dumps(shards[i]))
                deferred = i == 15
                (fixture / 'validation-summary.json').write_text(json.dumps(dict(valid=True, expected=[name], passed=[] if deferred else [name], errors=[], deferred=[name] if deferred else [])))
                events = []
                if deferred:
                    events.append(dict(Action='output', Package=package, Test=name, Output='    deferred_test.go:1: requires explicitly installed DataWeave toolchain\n'))
                events += [dict(Action='skip' if deferred else 'pass', Package=package, Test=name, Elapsed=2), dict(Action='pass', Package=package, Elapsed=2)]
                (fixture / 'events.json').write_text(''.join(json.dumps(row) + '\n' for row in events))
            worker = root / 'scripts/ci-go-test.sh'
            worker.write_text('#!/bin/bash\nif [ "$1" = apex-shard ]; then cp "$FIXTURES/fixture-$2/"* "$CI_APEXTEST_ARTIFACT_DIR/"; else printf "%s" "${CI_APEXTEST_HISTORY_COMPLETE:-}" > "$FLAG"; exec bash "$REAL" "$@"; fi\n')
            worker.chmod(0o755)
            options = self.capture_plan(root)['apextest-dispatch'][1]['env']
            env = {**os.environ, **options, 'FIXTURES': str(root), 'FLAG': str(root / 'flag'), 'REAL': str(ROOT / 'scripts/ci-go-test.sh'), 'GITHUB_EVENT_NAME': 'pull_request'}
            env.pop('GLADE_DATAWEAVE_APEX_TEST_HOME', None)
            # Exercise the helper's default history path inside this fixture,
            # even when the surrounding CI job supplies a shared history path.
            env.pop('CI_APEXTEST_HISTORY_PATH', None)
            result = subprocess.run(['bash', str(root / 'scripts/apextest-shards.sh'), str(root / 'evidence')], env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((root / 'flag').read_text(), '1')
            history = json.loads((root / 'evidence/duration-history.json').read_text())
            self.assertTrue(history['complete'])
            self.assertEqual([item['name'] for item in history['tests']], names)
            self.assertEqual(history['tests'][-1]['durationMillis'], 0)

    def test_new_shards_and_history_use_workflow_routes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            calls = self.capture_plan(root)
            for group in ('gladecli', 'vm'):
                count = runner.SHARDS[group]
                for index in range(count):
                    command, options = calls[f'{group}-{index}']
                    self.assertEqual(command, ['scripts/ci-go-test.sh', group + '-shard', str(index)])
                    self.assertEqual(options['env'][f'CI_{group.upper()}_ARTIFACT_DIR'], str(root / f'{group}-{index}'))
                    self.assertNotIn('CI_TEST_BIN_DIR', options['env'])
                    self.assertEqual(options['env'][f'CI_{group.upper()}_SHARDS'], str(count))
                self.assertNotIn(f'{group}-{count}', calls)
                command, options = calls[group + '-history']
                self.assertEqual(command, ['scripts/ci-go-test.sh', group + '-history-refresh', str(root / (group + '-history.json')), *[str(root / f'{group}-{i}') for i in range(count)]])
                if group == 'vm':
                    self.assertEqual(options['env']['CI_VM_HISTORY_COMPLETE'], '1')

    def test_smoke_runtime_handles_log_paths_with_spaces(self):
        with tempfile.TemporaryDirectory(prefix='ci local ') as directory:
            base = pathlib.Path(directory)
            logroot = base / 'logs with spaces'
            logroot.mkdir()
            calls = self.capture_plan(logroot)
            script, kwargs = calls['smoke-runtime']
            fake = base / 'project'
            (fake / 'scripts').mkdir(parents=True)
            (fake / 'bin').mkdir()
            go = fake / 'bin/go'
            go.write_text('#!/bin/bash\nprintf binary > "$3"\n')
            go.chmod(0o755)
            smoke = fake / 'scripts/smoke-runtime.sh'
            smoke.write_text('#!/bin/bash\ntest "$#" -eq 1 && test -f "$1"\n')
            smoke.chmod(0o755)
            actual = runner.Runner(logroot, 1)
            actual.env['PATH'] = str(fake / 'bin') + os.pathsep + os.environ['PATH']
            with mock.patch.object(runner, 'ROOT', fake):
                actual.shell('smoke-check', script, **kwargs)
            self.assertEqual(actual.results['smoke-check'][0], 'PASS', actual.results['smoke-check'][1].read_text())

    def test_candidate_toolchain_is_selected_for_browser_and_shards(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            calls = self.capture_plan(root)
            r = runner.Runner(root, 1)
            self.assertEqual(r.env['GLADE_ROOT'], str(ROOT))
            self.assertEqual(r.env['GLADE_LWC_PLAYWRIGHT_MODULE'], str(ROOT / 'lwcruntime/node_modules/playwright'))
            self.assertEqual(r.env['XDG_DATA_HOME'], str(root / 'xdg-data'))
            for name in ['lwc-compile-0', 'visualforce-0']:
                self.assertNotEqual(calls[name][1]['env'].get('GLADE_HOME'), str(ROOT))
                self.assertEqual(r.env['GLADE_ROOT'], str(ROOT))

    def test_site_rejects_existing_server_and_failed_preview(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            calls = self.capture_plan(root)
            script, kwargs = calls['site']
            self.assertIn('CI=1', script)
            self.assertIn('NODE_OPTIONS=', script)
            self.assertIn('kill -0', script)
            self.assertIn('Built site served at', script)
            self.assertIn('scripts/ci-site-preview.cjs', script)
            self.assertIn('preview.log', script)

    def test_site_smoke_fails_when_preview_cannot_start(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            calls = self.capture_plan(root)
            script, kwargs = calls['site']
            bindir = root / 'bin'
            bindir.mkdir()
            for name, body in [('npm', 'exit 0'), ('node', 'echo port-busy >&2; exit 1'), ('curl', 'exit 0'), ('sleep', 'exit 0')]:
                executable = bindir / name
                executable.write_text('#!/bin/bash\n' + body + '\n')
                executable.chmod(0o755)
            actual = runner.Runner(root, 1)
            actual.env['PATH'] = str(bindir) + os.pathsep + os.environ['PATH']
            actual.shell('site-check', script, **kwargs)
            self.assertEqual(actual.results['site-check'][0], 'FAIL')

    def test_site_browser_preload_handles_checkout_path_with_spaces(self):
        with tempfile.TemporaryDirectory(prefix='checkout space ') as directory:
            project = pathlib.Path(directory)
            logs = project / 'logs'
            logs.mkdir()
            script, kwargs = self.capture_plan(logs)['site']
            bindir = project / 'bin'
            bindir.mkdir()
            npm = bindir / 'npm'
            npm.write_text('#!/bin/bash\nif [[ "$*" == *test:browser* ]]; then printf "%s" "$NODE_OPTIONS" > "$CI_LOCAL_LOG_ROOT/node-options"; fi\n')
            npm.chmod(0o755)
            node = bindir / 'node'
            node.write_text('#!/bin/bash\nexit 1\n')
            node.chmod(0o755)
            actual = runner.Runner(logs, 1)
            actual.env['PATH'] = str(bindir) + os.pathsep + os.environ['PATH']
            with mock.patch.object(runner, 'ROOT', project):
                actual.shell('site-check', script, **kwargs)
            self.assertEqual((logs / 'node-options').read_text(), '--require="' + str(project.resolve() / 'scripts/ci-site-preview.cjs') + '"')

    def test_browser_check_does_not_resolve_parent_playwright(self):
        with tempfile.TemporaryDirectory() as directory:
            project = pathlib.Path(directory)
            (project / 'lwcruntime/node_modules').mkdir(parents=True)
            r = runner.Runner(project, 1)
            r.node = True
            with mock.patch.object(runner, 'ROOT', project), mock.patch.object(runner.subprocess, 'run', side_effect=AssertionError('must not resolve outside checkout')):
                self.assertFalse(r.browser_available('lwcruntime'))

    def test_shard_inventory_matches_workflow(self):
        self.assertEqual(runner.SHARDS, {'apextest': 16, 'sema': 2, 'lwc-compile': 8, 'visualforce': 4, 'gladecli': 2, 'vm': 3})


if __name__ == '__main__':
    unittest.main()
