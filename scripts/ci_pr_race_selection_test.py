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


if __name__ == "__main__":
    unittest.main()
