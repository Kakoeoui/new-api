from contextlib import redirect_stdout
import io
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import check_github_files
from check_github_files import forbidden_path, private_content, public_legacy_env


class GithubBoundaryTests(unittest.TestCase):
    def test_private_paths_are_blocked_but_source_and_examples_work(self):
        for path in ['private/shop/file.json', 'nested/.env.customer', 'shop.sql', 'one-api.db.before-shop-1', 'data.sqlite3', 'model-catalog.json', 'perfShops/config.json']:
            self.assertTrue(forbidden_path(path), path)
        for path in ['web/src/main.tsx', 'controller/notification_content.go', '.env.customer.example', 'bin/migration_v0.2-v0.3.sql']:
            self.assertFalse(forbidden_path(path), path)

    def test_renamed_backups_and_known_credentials_are_blocked(self):
        secret = b'synthetic-test-credential-only'
        self.assertTrue(private_content(b'prefix ' + secret, {secret}, set()))
        self.assertTrue(private_content(b'SQLite format 3\0content', set(), set()))
        self.assertTrue(private_content(b'-- PostgreSQL database dump\n', set(), set()))
        self.assertFalse(private_content(b'ordinary source code', {secret}, set()))

    def test_historical_env_exception_only_allows_the_empty_upstream_template(self):
        template = b'VITE_REACT_APP_SERVER_URL=\nVITE_REACT_APP_VERSION=\n'
        self.assertTrue(public_legacy_env(template))
        self.assertFalse(public_legacy_env(template + b'API_KEY=synthetic-secret\n'))
        self.assertFalse(public_legacy_env(template.replace(b'URL=\n', b'URL=private-value\n')))

    def test_safe_working_copy_cannot_hide_a_staged_secret(self):
        with tempfile.TemporaryDirectory() as directory:
            repo = Path(directory)
            subprocess.run(['git', 'init', '--quiet', directory], check=True)
            source = repo / 'settings.txt'
            source.write_text('synthetic-staged-credential-only')
            subprocess.run(['git', '-C', directory, 'add', '.'], check=True)
            source.write_text('safe working copy')
            output = io.StringIO()
            with patch.object(check_github_files, 'REPO', repo), patch.object(check_github_files, 'private_fingerprints', return_value=({b'synthetic-staged-credential-only'}, set())), patch('sys.argv', ['check_github_files.py']), redirect_stdout(output):
                self.assertEqual(check_github_files.main(), 1)
            self.assertIn('settings.txt', output.getvalue())
            self.assertNotIn('synthetic-staged-credential-only', output.getvalue())


if __name__ == '__main__':
    unittest.main()
