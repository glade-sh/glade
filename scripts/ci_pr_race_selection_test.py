import json
import pathlib
import subprocess
import tempfile
import unittest


class RaceSelectionTest(unittest.TestCase):
    def test_intersects_affected_concurrency_packages(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            for name, source in {
                "sema": 'package sema\nimport "sync"\nvar mu sync.Mutex\n',
                "lsp": 'package lsp\nfunc run() { go work() }\n',
                "plain": 'package plain\nfunc f() {}\n',
                "unaffected": 'package unaffected\nvar ch chan int\n',
            }.items():
                directory = root / name
                directory.mkdir()
                (directory / "code.go").write_text(source)
            graph = [{"ImportPath": "example/" + name, "Dir": str(root / name)}
                     for name in ("sema", "lsp", "plain", "unaffected")]
            graphfile = root / "graph.json"
            graphfile.write_text(json.dumps(graph))
            selector = pathlib.Path(__file__).with_name("ci-pr-race-select.py")
            def select(packages):
                result = subprocess.run(["python3", str(selector), "--graph", str(graphfile)],
                                        input=json.dumps(packages), text=True,
                                        capture_output=True, check=True)
                return json.loads(result.stdout)
            self.assertEqual(select(["example/lsp"]), ["example/lsp"])
            self.assertEqual(select([]), [])  # docs only
            # sema changes also reach lsp through the existing PR selector.
            self.assertEqual(select(["example/sema", "example/lsp", "example/plain"]),
                             ["example/lsp", "example/sema"])

class RaceSelectionIntegrationTest(unittest.TestCase):
    def test_docs_fast_path_rejects_package_and_fixture_paths(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            (root / 'docs/pkg').mkdir(parents=True)
            (root / 'docs/pkg/code.go').write_text('package pkg')
            selector = pathlib.Path(__file__).with_name('ci-pr-race-select.py')
            def fast(paths):
                return subprocess.run(['python3', str(selector), '--docs-only', '--root', str(root)],
                                      input=paths, text=True, capture_output=True, check=True).stdout.strip()
            self.assertEqual(fast('docs/guide.md\nREADME.md\n'), 'true')
            self.assertEqual(fast('docs/pkg/guide.md\n'), 'false')
            self.assertEqual(fast('docs/new/code.go\n'), 'false')
            self.assertEqual(fast('docs/testdata/fixture.md\n'), 'false')
            self.assertEqual(fast(''), 'false')

    def test_existing_selector_docs_and_indirect_sema(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            names = ("sema", "lsp", "plain")
            for name in names:
                (root / name).mkdir()
                (root / name / "x.go").write_text(
                    'package x\n' + ('import "sync"\n' if name != "plain" else ''))
            parser_dir = root / 'third_party/glade-apex-parser'
            parser_dir.mkdir(parents=True)
            graph = [{"ImportPath": "example/" + name, "Dir": str(root / name),
                      "Imports": ["example/sema"] if name == "lsp" else []}
                     for name in names]
            parser_graph = [{"ImportPath": "github.com/glade-sh/apex-parser",
                             "Dir": str(parser_dir)}]
            for filename, data in (("graph.json", graph), ("parser.json", parser_graph),
                                   ("manifest.json", {"lanes": {"all": [
                                       "example/" + name for name in names]}})):
                (root / filename).write_text(json.dumps(data))
            scripts = pathlib.Path(__file__).parent
            def select(paths):
                affected = subprocess.run([
                    'python3', str(scripts / 'ci-pr-select.py'), '--root', str(root),
                    '--graph', str(root / 'graph.json'), '--parser-graph', str(root / 'parser.json'),
                    '--manifest', str(root / 'manifest.json')], input=paths, text=True,
                    capture_output=True, check=True).stdout
                result = subprocess.run(['python3', str(scripts / 'ci-pr-race-select.py'),
                                         '--graph', str(root / 'graph.json')],
                                        input=affected, text=True, capture_output=True, check=True)
                return json.loads(result.stdout)
            self.assertEqual(select('docs/guide.md\n'), [])
            self.assertEqual(select('lsp/x.go\n'), ['example/lsp'])
            self.assertEqual(select('sema/x.go\n'), ['example/lsp', 'example/sema'])


if __name__ == "__main__":
    unittest.main()
