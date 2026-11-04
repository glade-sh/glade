"""Owned Git fixtures only; never invokes CodeQL or the network."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("ci_codeql", Path(__file__).with_name("ci-codeql.py"))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class LocalDiffTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        isolated = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
        isolated.update({
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "Fixture", "GIT_AUTHOR_EMAIL": "fixture@example.invalid",
            "GIT_COMMITTER_NAME": "Fixture", "GIT_COMMITTER_EMAIL": "fixture@example.invalid",
            "GITHUB_WORKSPACE": str(self.repo), "GITHUB_EVENT_NAME": "pull_request",
            "GITHUB_EVENT_PATH": str(self.root / "event.json"),
            "CODEQL_ACTION_TEMP": str(self.root / "temp"),
        })
        self.env = patch.dict(os.environ, isolated, clear=True)
        self.env.start()
        self.addCleanup(self.env.stop)
        (self.root / "temp").mkdir()
        self.git("init", "--template=", "-q")
        self.write("source.go", "package fixture\n// old\n// tail\n")
        self.base = self.commit()
        self.head = self.base
        self.event()

    def git(self, *args):
        return subprocess.check_output(["git", "-c", "core.hooksPath=" + os.devnull,
                                        "-C", str(self.repo), *args], stderr=subprocess.PIPE).decode().strip()

    def write(self, name, body):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_bytes(body if isinstance(body, bytes) else body.encode())

    def commit(self):
        self.git("add", "--all")
        self.git("commit", "-qm", "owned fixture", "--allow-empty")
        return self.git("rev-parse", "HEAD")

    def event(self, analyzed=None):
        os.environ["GITHUB_SHA"] = analyzed or self.head
        (self.root / "event.json").write_text(json.dumps({"pull_request": {
            "base": {"sha": self.base}, "head": {"sha": self.head}}}))

    def rows(self):
        return ci.local_diff_ranges()

    def test_missing_action_ranges_regression(self):
        self.write("source.go", "package fixture\n// new\n// tail\n")
        self.head = self.commit()
        self.event()
        pack = ci.diff_pack("fast")
        self.assertIsNotNone(pack, "missing API ranges must use the complete local diff")
        data = json.loads((pack / "diff-ranges.yml").read_text())["extensions"][0]["data"]
        self.assertEqual(data, [[str(self.repo / "source.go"), 2, 2]])

    def test_large_file_counts_keep_last_file(self):
        for count in (299, 300, 301, 1032):
            with self.subTest(count=count):
                self.git("reset", "--hard", self.base)
                for i in range(count):
                    self.write(f"src/{i:04}.go", "package fixture\n// owned source or sink\n")
                self.head = self.commit()
                self.event()
                rows = self.rows()
                self.assertEqual(len(rows), count)
                self.assertIn({"path": f"src/{count-1:04}.go", "startLine": 1, "endLine": 2}, rows)

    def test_merge_tree_coordinates_include_base_shift(self):
        self.git("checkout", "-qb", "pr")
        self.write("source.go", "package fixture\n// old\n// PR tail\n")
        self.head = self.commit()
        self.git("checkout", "-qb", "target", self.base)
        self.write("source.go", "// base added line\npackage fixture\n// old\n// tail\n")
        self.base = self.commit()
        self.git("merge", "--no-ff", "-m", "owned merge", "pr")
        merged = self.git("rev-parse", "HEAD")
        self.event(merged)
        rows = self.rows()
        self.assertIn({"path": "source.go", "startLine": 4, "endLine": 4}, rows)
        self.assertIn({"path": "source.go", "startLine": 1, "endLine": 1}, rows)

    def test_unusual_paths_rename_delete_and_binary(self):
        self.git("mv", "source.go", "renamed.go")
        paths = ["space name.go", "quote'\".go", "tab\tname.go", "line\nname.go", ":(glob)literal.go", "unicode-é.go"]
        for p in paths:
            self.write(p, "package fixture\n")
        self.write("binary.dat", b"\x00\x01\x02")
        self.head = self.commit()
        self.event()
        rows = self.rows()
        names = {r["path"] for r in rows}
        self.assertNotIn("source.go", names)
        self.assertTrue(set(paths + ["renamed.go", "binary.dat"]).issubset(names))
        self.assertIn({"path": "binary.dat", "startLine": 0, "endLine": 0}, rows)

    def test_deletion_only_hunk_uses_whole_surviving_file(self):
        self.write("source.go", "package fixture\n// tail\n")
        self.head = self.commit()
        self.event()
        self.assertEqual(self.rows(), [{"path": "source.go", "startLine": 0, "endLine": 0}])

    def test_verified_empty_diff(self):
        self.assertEqual(self.rows(), [])
        pack = ci.diff_pack("fast")
        self.assertEqual(json.loads((pack / "diff-ranges.yml").read_text())["extensions"][0]["data"], [["", 0, 0]])

    def test_wrong_checkout_and_missing_commit_fail_closed(self):
        self.write("source.go", "new\n")
        self.head = self.commit()
        self.event()
        os.environ["GITHUB_SHA"] = self.base
        with self.assertRaises(ValueError): self.rows()
        self.event()
        self.head = "f" * 40
        self.event()
        with self.assertRaises((ValueError, subprocess.CalledProcessError)): self.rows()

    def test_unrelated_merge_rejected(self):
        self.git("checkout", "-qb", "pr")
        self.write("pr.go", "package fixture\n")
        self.head = self.commit()
        self.git("checkout", "-qb", "other", self.base)
        self.write("other.go", "package fixture\n")
        self.commit()
        self.git("merge", "--no-ff", "-m", "merge", "pr")
        self.event(self.git("rev-parse", "HEAD"))
        with self.assertRaises(ValueError): self.rows()

    def test_dirty_or_untracked_go_rejected(self):
        self.write("source.go", "changed\n")
        with self.assertRaises(ValueError): self.rows()
        self.git("checkout", "--", "source.go")
        self.write("untracked.go", "package fixture\n")
        with self.assertRaises(ValueError): self.rows()

    def test_full_and_non_pr_do_not_create_pack(self):
        self.assertIsNone(ci.diff_pack("full"))
        os.environ["GITHUB_EVENT_NAME"] = "push"
        self.assertIsNone(ci.diff_pack("fast"))

    def test_existing_action_ranges_preserved(self):
        (self.root / "temp/pr-diff-range.json").write_text('[{"path":"source.go","startLine":2,"endLine":3}]')
        with patch.object(ci, "local_diff_ranges", side_effect=AssertionError("must not run")):
            self.assertIsNotNone(ci.diff_pack("fast"))

    def test_failure_does_not_create_empty_pack(self):
        os.environ["GITHUB_SHA"] = "not-a-sha"
        with self.assertRaises(ValueError): ci.diff_pack("fast")
        self.assertFalse((self.root / "temp/pr-diff-range").exists())

    def test_changed_symlink_rejected(self):
        (self.repo / "link.go").symlink_to("source.go")
        self.head = self.commit()
        self.event()
        with self.assertRaises(ValueError): self.rows()

    def test_ignored_untracked_go_rejected(self):
        self.write(".gitignore", "generated.go\n")
        self.head = self.commit()
        self.event()
        self.write("generated.go", "package fixture\n")
        with self.assertRaises(ValueError): self.rows()

    def test_shallow_history_rejected(self):
        self.write("source.go", "new\n")
        self.head = self.commit()
        shallow = self.root / "shallow"
        subprocess.run(["git", "clone", "--template=", "-q", "--depth", "1", self.repo.as_uri(), str(shallow)], check=True)
        os.environ["GITHUB_WORKSPACE"] = str(shallow)
        self.event()
        with self.assertRaises((ValueError, subprocess.CalledProcessError)): self.rows()

    def test_external_diff_is_not_executed(self):
        # A deliberately failing configured driver would fail the diff if run.
        self.git("config", "diff.external", "/does-not-exist-owned-fixture")
        self.write("source.go", "new\n")
        self.head = self.commit()
        self.event()
        self.assertTrue(self.rows())

    def test_malformed_existing_ranges_are_not_empty_success(self):
        (self.root / "temp/pr-diff-range.json").write_text('{"truncated":true}')
        with self.assertRaises(ValueError): ci.diff_pack("fast")

    def test_shared_deadline_exhaustion_is_failure(self):
        with self.assertRaises(subprocess.TimeoutExpired):
            ci.local_diff_ranges(time.monotonic() - 1)

    def test_analyze_passes_local_pack_to_cli(self):
        self.write("source.go", "package fixture\n// changed\n// tail\n")
        self.head = self.commit()
        self.event()
        fake = self.root / "codeql"
        fake.write_text("""#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
phase = sys.argv[2]
Path(os.environ['CODEQL_CALL_DIR'], phase + '.json').write_text(json.dumps(sys.argv))
if phase == 'interpret-results':
    output = next(a.split('=', 1)[1] for a in sys.argv if a.startswith('--output='))
    Path(output).write_text(json.dumps({'version':'2.1.0', 'runs':[{'tool':{'driver':{'name':'CodeQL'}}, 'results':[]}]}))
""")
        fake.chmod(0o700)
        os.environ["CODEQL_CALL_DIR"] = str(self.root)
        os.environ["GITHUB_OUTPUT"] = str(self.root / "output")
        os.environ["GITHUB_ENV"] = str(self.root / "env")
        budget = self.root / "budget.json"
        budget.write_text('{"analyze_seconds":30,"query_seconds":20}')
        args = SimpleNamespace(budget=str(budget), output=str(self.root / "result.sarif"),
                               codeql=str(fake), database=str(self.root / "db"), profile="fast")
        ci.analyze(args)
        query = json.loads((self.root / "run-queries.json").read_text())
        interpret = json.loads((self.root / "interpret-results.json").read_text())
        self.assertIn("--extension-packs=codeql-action/pr-diff-range", query)
        self.assertIn("--ram=8192", query)
        self.assertIn("--sarif-run-property=incrementalMode=diff-informed", interpret)
        self.assertIn("completed=true", (self.root / "output").read_text())

    def test_bad_provenance_fails_cli_before_analysis(self):
        os.environ["GITHUB_SHA"] = "invalid"
        os.environ["GITHUB_OUTPUT"] = str(self.root / "output")
        os.environ["GITHUB_ENV"] = str(self.root / "env")
        budget = self.root / "budget.json"
        budget.write_text('{"analyze_seconds":30,"query_seconds":20}')
        result = self.root / "stale.sarif"
        result.write_text("stale")
        run = subprocess.run(["python3", str(Path(ci.__file__)), "analyze", "--profile", "fast",
                              "--codeql", "/must-not-be-run", "--database", str(self.root),
                              "--output", str(result), "--budget", str(budget)], capture_output=True)
        self.assertNotEqual(run.returncode, 0)
        self.assertFalse(result.exists())
        self.assertNotIn("completed=true", (self.root / "output").read_text())
        self.assertIn("JOB_STATUS_FAILURE", (self.root / "env").read_text())


if __name__ == "__main__":
    unittest.main()
