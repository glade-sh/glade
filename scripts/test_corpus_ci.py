import importlib.util
import json
import pathlib
import tempfile
import unittest
from unittest import mock

SCRIPT = pathlib.Path(__file__).with_name('corpus_ci.py')
spec = importlib.util.spec_from_file_location('corpus_ci', SCRIPT)
corpus_ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(corpus_ci)


def sample():
    return {'schema': 1, 'repos': [
        {'id': f'public-{i}', 'url': f'https://github.com/example/project-{i}.git',
         'pin': str(i) * 40, 'license_sha256': 'a' * 64,
         'golden': {'overlay': {'config': {}}, 'glade': {'sourceApi': '63.0'},
                    'counts': {'classes': 1, 'tests': 1, 'passed': 1}, 'wallSeconds': 1}}
        for i in range(1, 5)]}


class CorpusCITest(unittest.TestCase):
    def test_manifest_rejects_unpinned_or_duplicate_inputs(self):
        for mutate in (
            lambda x: x['repos'][0].update(pin='main'),
            lambda x: x['repos'][0].update(url='file:///tmp/project'),
            lambda x: x['repos'][1].update(id=x['repos'][0]['id']),
            lambda x: x['repos'][0].update(license_sha256=''),
            lambda x: x['repos'].pop(),
        ):
            with self.subTest(mutate=mutate):
                data = sample()
                mutate(data)
                with self.assertRaises(ValueError):
                    corpus_ci.validate_manifest(data)

    def test_fresh_config_matches_golden_identity(self):
        data = sample()
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            clones = [root / f'clone-{i}' for i in range(4)]
            config = corpus_ci.write_config(data, root / 'config', clones)
            self.assertEqual(len(config['repos']), 4)
            for row, clone in zip(data['repos'], clones):
                entry = next(x for x in config['repos'] if x['name'] == row['id'])
                golden = json.loads((root / 'config' / 'goldens' / (row['id'] + '.json')).read_text())
                self.assertEqual(entry['pin'], golden['pin'])
                self.assertEqual(entry['overlay'], golden['overlay'])
                self.assertEqual(entry['clone'], [str(clone)])
                self.assertEqual(golden['repo'], row['id'])

    def test_golden_rejects_unclassified_or_private_details(self):
        for mutation in (
            lambda row: row['golden'].update(failing=[{'test': 'secret', 'status': 'unclassified'}]),
            lambda row: row['golden'].update(diagnostics=[{'file': 'secret', 'status': 'known-gap'}]),
        ):
            data = sample()
            mutation(data['repos'][0])
            with self.assertRaises(ValueError):
                corpus_ci.validate_manifest(data)


    def test_runner_exit_code_is_propagated(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            runner = root / 'runner'
            glade = root / 'glade'
            runner.touch()
            glade.touch()
            env = {'CORPUS_A_MANIFEST': json.dumps(sample()), 'CORPUS_RUNNER': str(runner),
                   'GLADE_BINARY': str(glade), 'RUNNER_TEMP': str(root)}
            with mock.patch.dict('os.environ', env, clear=True), \
                 mock.patch.object(corpus_ci, 'checkout'), \
                 mock.patch.object(corpus_ci.subprocess, 'run') as run:
                run.return_value.returncode = 1
                self.assertEqual(corpus_ci.main(), 1)
                run.return_value.returncode = 0
                self.assertEqual(corpus_ci.main(), 0)


if __name__ == '__main__':
    unittest.main()
