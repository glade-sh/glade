#!/usr/bin/env python3
"""Generate the embedded standard report catalogs from native captures."""
import json
from pathlib import Path
root = Path(__file__).resolve().parent.parent
source = root / 'internal/resource/testdata/report_rows_capture/type-catalog-three'
target = root / 'internal/vm/report_catalog_templates'
for name in ('account', 'contact', 'opportunity'):
    data = json.loads((source / f'baseline-{name}-62.json').read_text())
    (target / f'{name}.json').write_text(json.dumps(data, separators=(',', ':'), sort_keys=True) + '\n')

# Fingerprints match the complete effective permission sets captured in the
# synthetic security matrix without treating unobserved combinations alike.
import hashlib
permissions = root / 'internal/resource/testdata/report_rows_capture/security-permission-boundaries'
fingerprints = {}
for state in ('plain', 'probe', 'viewall', 'modifyall'):
    flags = json.loads((permissions / f'effective-{state}.json').read_text())
    canonical = '\n'.join(sorted(flag.lower() for flag in flags))
    fingerprints[state] = hashlib.sha256(canonical.encode()).hexdigest()
(target / 'security-fingerprints.json').write_text(json.dumps(fingerprints, sort_keys=True, separators=(',', ':')) + '\n')
