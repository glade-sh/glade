import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

SCRIPT = Path(__file__).with_name('generate-standard-feature-describes.py')
spec = importlib.util.spec_from_file_location('feature_describes', SCRIPT)
generator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(generator)


class FeatureDescribeGeneratorTest(unittest.TestCase):
    def test_check_detects_drift_without_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'sidecar.gz'
            output.write_bytes(b'drift')
            with mock.patch.object(generator, 'OUTPUT', output), mock.patch.object(sys, 'argv', [str(SCRIPT), '--check']):
                with self.assertRaises(SystemExit) as error:
                    generator.main()
                self.assertNotEqual(error.exception.code, 0)
            self.assertEqual(output.read_bytes(), b'drift')
