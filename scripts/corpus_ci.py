#!/usr/bin/env python3
"""Prepare clean, pinned public corpus inputs and invoke the shared runner."""
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import hashlib


def validate_manifest(data):
    if not isinstance(data, dict) or data.get('schema') != 1:
        raise ValueError('invalid manifest schema')
    repos = data.get('repos')
    if not isinstance(repos, list) or not 4 <= len(repos) <= 6:
        raise ValueError('expected four to six projects')
    ids = set()
    for row in repos:
        if not isinstance(row, dict) or set(row) != {'id', 'url', 'pin', 'license_sha256', 'golden'}:
            raise ValueError('invalid project record')
        name = row['id']
        if not isinstance(name, str) or not re.fullmatch(r'public-[1-6]', name) or name in ids:
            raise ValueError('invalid project identifier')
        ids.add(name)
        if not isinstance(row['url'], str) or not re.fullmatch(r'https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+\.git', row['url']):
            raise ValueError('invalid public origin')
        if not isinstance(row['pin'], str) or not re.fullmatch(r'[0-9a-f]{40}', row['pin']):
            raise ValueError('project is not pinned')
        if not isinstance(row['license_sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', row['license_sha256']):
            raise ValueError('license is not pinned')
        golden = row['golden']
        if not isinstance(golden, dict) or set(golden) != {'overlay', 'glade', 'counts', 'wallSeconds'}:
            raise ValueError('golden must contain only the clean baseline fields')
        if golden['overlay'] != {'config': {}} or set(golden['glade']) != {'sourceApi'}:
            raise ValueError('unsupported overlay or API')
        if not re.fullmatch(r'[0-9]+\.0', str(golden['glade']['sourceApi'])):
            raise ValueError('invalid API')
        counts = golden['counts']
        if not isinstance(counts, dict) or set(counts) != {'classes', 'tests', 'passed'}:
            raise ValueError('invalid counts')
        if any(type(v) is not int or v < 0 for v in counts.values()) or counts['tests'] == 0 or counts['passed'] != counts['tests']:
            raise ValueError('expected a complete clean test baseline')
        if type(golden['wallSeconds']) not in (int, float) or not 0 < golden['wallSeconds'] < 7200:
            raise ValueError('invalid baseline duration')
    return repos


def write_config(data, config_dir, clones):
    repos = validate_manifest(data)
    if len(clones) != len(repos):
        raise ValueError('clone count differs from manifest')
    golden_dir = config_dir / 'goldens'
    golden_dir.mkdir(parents=True)
    config = {'schemaVersion': 1, 'depSources': {}, 'repos': []}
    for row, clone in zip(repos, clones):
        golden = {'repo': row['id'], 'pin': row['pin'], **row['golden']}
        (golden_dir / (row['id'] + '.json')).write_text(json.dumps(golden) + '\n')
        config['repos'].append({'name': row['id'], 'tier': 'A', 'clone': [str(clone)],
                                'pin': row['pin'], 'overlay': row['golden']['overlay']})
    (config_dir / 'tiers.json').write_text(json.dumps(config) + '\n')
    return config


def checkout(row, root):
    root.mkdir()
    def git(*args):
        subprocess.run(['git', '-C', str(root), *args], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                       env={**os.environ, 'GIT_TERMINAL_PROMPT': '0'})
    git('init', '-q')
    git('fetch', '-q', '--no-tags', '--depth=1', row['url'], row['pin'])
    git('checkout', '-q', '--detach', 'FETCH_HEAD')
    head = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
    license_file = root / 'LICENSE'
    if head != row['pin'] or license_file.is_symlink() or not license_file.is_file():
        raise ValueError('pinned checkout or license is unavailable')
    if hashlib.sha256(license_file.read_bytes()).hexdigest() != row['license_sha256']:
        raise ValueError('pinned license differs from reviewed license')


def main():
    try:
        repos = validate_manifest(json.loads(os.environ['CORPUS_A_MANIFEST']))
        runner = pathlib.Path(os.environ['CORPUS_RUNNER']).resolve(strict=True)
        glade = pathlib.Path(os.environ['GLADE_BINARY']).resolve(strict=True)
        with tempfile.TemporaryDirectory(prefix='corpus-a-', dir=os.environ.get('RUNNER_TEMP')) as tmp:
            root = pathlib.Path(tmp)
            clones = []
            for row in repos:
                clone = root / row['id']
                checkout(row, clone)
                clones.append(clone)
            write_config({'schema': 1, 'repos': repos}, root / 'config', clones)
            return subprocess.run([str(runner), str(glade), str(root / 'config')], check=False).returncode
    except (KeyError, ValueError, OSError, json.JSONDecodeError, subprocess.SubprocessError):
        print('CORPUS-INFRA: invalid manifest, pinned checkout, license, or runner', file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(main())
