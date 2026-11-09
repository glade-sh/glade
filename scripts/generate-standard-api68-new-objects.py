#!/usr/bin/env python3
"""Generate the API 68 new-object names and profile features from the sidecar.

standardAPI68CapturedNewObject only needs the names of objects a profile
captured with a 200 describe at API 68, and StandardAPI68DescribeProfile only
needs each profile's feature list. Decoding the whole sidecar for either costs
about 40 ms and 28 MB per process, so both are generated here and the bundle
is decoded only for API 68 projects.
"""
import argparse
import gzip
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SIDECAR = ROOT / 'internal/storage/standard_api68_describe.json.gz'
OUTPUT = ROOT / 'internal/storage/standard_api68_new_objects_generated.go'


def load_bundle(sidecar):
    bundle = json.loads(gzip.decompress(sidecar.read_bytes()))
    if bundle.get('version') != 1:
        raise ValueError('unsupported API68 describe sidecar version')
    return bundle


def new_object_names(bundle):
    names = set()
    for profile in bundle['profiles'].values():
        for name, captured in profile['objects'].items():
            if captured['status'] == 200:
                names.add(name)
    return sorted(names)


def profile_features(bundle):
    return {name: list(profile['features']) for name, profile in sorted(bundle['profiles'].items())}


def render(names, features):
    lines = [
        '// Code generated from standard_api68_describe.json.gz by scripts/generate-standard-api68-new-objects.py; DO NOT EDIT.',
        '',
        'package storage',
        '',
        '// standardAPI68NewObjectNames lists the objects some captured profile',
        '// described with status 200 at API 68. Keys are lower-cased API names.',
        'var standardAPI68NewObjectNames = map[string]struct{}{',
    ]
    keys = [json.dumps(name.lower()) for name in names]
    width = max(len(key) for key in keys) if keys else 0
    for key in keys:
        # gofmt aligns map literal values; pad so the output is already clean.
        lines.append('\t%s:%s {},' % (key, ' ' * (width - len(key))))
    lines.append('}')
    lines += [
        '',
        '// standardAPI68ProfileFeatures is each captured profile\'s feature list, in',
        '// sidecar order. StandardAPI68DescribeProfile derives signatures from it.',
        'var standardAPI68ProfileFeatures = map[string][]string{',
    ]
    keys = [json.dumps(name) for name in features]
    width = max(len(key) for key in keys) if keys else 0
    for key, name in zip(keys, features):
        values = ', '.join(json.dumps(feature) for feature in features[name])
        lines.append('\t%s:%s {%s},' % (key, ' ' * (width - len(key)), values))
    lines.append('}')
    return '\n'.join(lines) + '\n'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sidecar', type=Path, default=SIDECAR)
    parser.add_argument('--output', type=Path, default=OUTPUT)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    bundle = load_bundle(args.sidecar)
    rendered = render(new_object_names(bundle), profile_features(bundle))
    if args.check:
        if not args.output.exists() or args.output.read_text() != rendered:
            raise SystemExit('API68 generated names differ')
        print('verified')
    else:
        args.output.write_text(rendered)
        print('wrote', args.output)


if __name__ == '__main__':
    main()
