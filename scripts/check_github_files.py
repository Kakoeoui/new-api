"""Check Git-visible files without printing private values or changing the index."""

import hashlib
import os
from pathlib import Path, PurePosixPath
import subprocess
import sys


REPO = Path(__file__).resolve().parent.parent
SQL_MIGRATIONS = {"bin/migration_v0.2-v0.3.sql", "bin/migration_v0.3-v0.4.sql"}
EXAMPLES = {".env.example", ".env.customer.example", ".env.perf.example"}
PRIVATE_FOLDERS = {"private", "datacustomer", "customer-private", "vps-private", "perfshops", "backups", ".local-tests", ".hallmark"}


def forbidden_path(name):
    path = PurePosixPath(name.lower())
    if PRIVATE_FOLDERS.intersection(path.parts):
        return True
    if path.name.startswith('.env') and name not in EXAMPLES:
        return True
    if path.suffix in {'.sql', '.dump', '.db', '.sqlite', '.sqlite3', '.bak', '.backup', '.pem', '.key'}:
        return name not in SQL_MIGRATIONS
    return '.db-' in path.name or '.db.' in path.name or path.name.startswith('model-catalog') and path.suffix == '.json'


def git(*args):
    return subprocess.check_output(['git', '-C', str(REPO), *args])


def private_fingerprints():
    directory = Path(os.environ.get('RVL_PRIVATE_DIR', REPO.parent / 'private' / 'shop'))
    secrets = set()
    fingerprints = set()
    if not directory.is_dir():
        return secrets, fingerprints
    for path in directory.iterdir():
        if not path.is_file():
            continue
        data = path.read_bytes()
        fingerprints.add(hashlib.sha256(data).digest())
        if path.name.startswith('.env'):
            for line in data.decode('utf-8-sig').splitlines():
                key, separator, value = line.partition('=')
                value = value.strip().strip('\"\'')
                if separator and not key.lstrip().startswith('#') and any(word in key.upper() for word in ('PASSWORD', 'SECRET', 'KEY', 'TOKEN', 'DSN')) and len(value) >= 16:
                    secrets.add(value.encode())
    dump = directory / 'shop-linux-move.sql'
    if dump.is_file():
        from model_catalog import read_dump
        for channel in read_dump(dump, ('models', 'channels'))['channels']:
            for value in (channel.get('key') or '').splitlines():
                if len(value) >= 16:
                    secrets.add(value.encode())
    return secrets, fingerprints


def private_content(data, secrets, fingerprints):
    return (data.startswith(b'SQLite format 3\0') or
            b'-- PostgreSQL database dump' in data[:512] or
            hashlib.sha256(data).digest() in fingerprints or
            any(value in data for value in secrets))


def public_legacy_env(data):
    # Upstream once tracked this empty frontend template; allow only its exact settings.
    return set(data.decode().splitlines()) == {'VITE_REACT_APP_SERVER_URL=', 'VITE_REACT_APP_VERSION='}


def main():
    secrets, fingerprints = private_fingerprints()
    names = set(git('ls-files', '-z', '--cached', '--others', '--exclude-standard').decode().split('\0')) - {''}
    staged = set(git('diff', '--cached', '--name-only', '--diff-filter=ACMR', '-z').decode().split('\0')) - {''}
    failures = set()
    for name in names:
        if forbidden_path(name):
            failures.add(name)
            continue
        path = REPO / name
        if path.is_file() and private_content(path.read_bytes(), secrets, fingerprints):
            failures.add(name)
    # Check the staged version too: a safe working copy must not hide an unsafe index.
    for name in staged:
        if forbidden_path(name) or private_content(git('show', ':' + name), secrets, fingerprints):
            failures.add(name)
    if '--history' in sys.argv:
        paths = git('log', '--all', '--format=', '--name-only', '--', '*.sql', '*.dump', '*.db', '*.sqlite*', '*.env*', '*private/*', '*datacustomer/*', '*model-catalog*.json').decode().splitlines()
        for name in set(paths) - {''}:
            if not forbidden_path(name):
                continue
            if name == 'web/.env':
                revisions = git('log', '--all', '--diff-filter=AM', '--format=%H', '--', name).decode().splitlines()
                if revisions and all(public_legacy_env(git('show', revision + ':' + name)) for revision in revisions):
                    continue
            failures.add('history: ' + name)
    if failures:
        print('BLOCKED: private data or forbidden files found (values are hidden):')
        for name in sorted(failures):
            print('  ' + name)
        print('Do not push. Move private files outside the checkout and remove them from the index/history as appropriate.')
        return 1
    print(f'PASS: {len(names)} Git-visible files and staged contents checked; private folders remain outside the repository.')
    print('Checks known local shop credentials and backup fingerprints; this is not an exhaustive secret scanner.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
