import importlib.util
import json
import sqlite3
import tempfile
import unittest
from contextlib import contextmanager
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("model_catalog", Path(__file__).with_name("model_catalog.py"))
catalog_tool = importlib.util.module_from_spec(spec)
spec.loader.exec_module(catalog_tool)


class ModelCatalogTests(unittest.TestCase):
    def setUp(self):
        scratch = ROOT / ".local-tests"
        scratch.mkdir(exist_ok=True)
        self.directory = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.directory.cleanup)
        self.path = Path(self.directory.name)
        self.dump = self.path / "backup.sql"
        self.catalog = self.path / "catalog.json"
        self.database = self.path / "test.db"
        self.dump.write_text(
            "DROP TABLE users;\n"
            "COPY public.users (id, email, password) FROM stdin;\n"
            "1\tPRIVATE_EMAIL\tPRIVATE_PASSWORD\n\\.\n"
            "COPY public.channels (id, key) FROM stdin;\n1\tPRIVATE_KEY\n\\.\n"
            "COPY public.vendors (id, name, description, icon, status, created_time, updated_time, deleted_at) FROM stdin;\n"
            "99\tVendor\tInfo\\tline\\nnext\tOpenAI\t1\t1\t2\t\\N\n\\.\n"
            "COPY public.models (id, model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, deleted_at, name_rule) FROM stdin;\n"
            "72\tmodel-a\tA model's description\tOpenAI\tChat\t99\t{}\t1\t1\t1\t2\t\\N\t0\n"
            "73\tremoved-model\tDeleted\t\t\t99\t{}\t1\t1\t1\t2\t2026-01-01\t0\n\\.\n",
            encoding="utf-8",
        )

    @contextmanager
    def open_database(self):
        db = sqlite3.connect(self.database)
        try:
            with db:
                yield db
        finally:
            db.close()

    def make_database(self):
        with self.open_database() as db:
            db.executescript("""
                CREATE TABLE vendors (id INTEGER PRIMARY KEY, name TEXT, description TEXT, icon TEXT, status INTEGER, created_time INTEGER, updated_time INTEGER, deleted_at TEXT);
                CREATE TABLE models (id INTEGER PRIMARY KEY, model_name TEXT, description TEXT, icon TEXT, tags TEXT, endpoints TEXT, status INTEGER, sync_official INTEGER, created_time INTEGER, updated_time INTEGER, name_rule INTEGER, vendor_id INTEGER, deleted_at TEXT);
                CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT);
                CREATE TABLE channels (id INTEGER PRIMARY KEY, key TEXT);
                CREATE TABLE options (key TEXT, value TEXT);
                INSERT INTO users VALUES (1, 'untouched');
                INSERT INTO channels VALUES (1, 'untouched');
                INSERT INTO options VALUES ('ModelRatio', 'untouched');
            """)

    def test_shop_import_preserves_customers_and_nonpricing_options(self):
        self.make_database()
        with self.open_database() as db:
            db.execute('CREATE TABLE abilities (channel_id INTEGER, enabled NUMERIC)')
            db.execute("INSERT INTO options VALUES ('SiteName', 'local-site')")
        with self.dump.open('a', encoding='utf-8') as source:
            source.write(
                'COPY public.abilities (channel_id, enabled) FROM stdin;\n1\tt\n\\.\n'
                'COPY public.options (key, value) FROM stdin;\n'
                'ModelRatio\t{"model-a":2}\nSiteName\tproduction-site\n'
                'PaymentKey\tDO_NOT_COPY\n\\.\n'
            )
        result = catalog_tool.import_shop(self.dump, self.database)
        self.assertEqual(result['channels'], 1)
        self.assertEqual(result['abilities'], 1)
        self.assertTrue(Path(result['backup']).exists())
        with self.open_database() as db:
            self.assertEqual(db.execute('SELECT email FROM users').fetchone()[0], 'untouched')
            self.assertEqual(db.execute('SELECT enabled FROM abilities').fetchone()[0], 1)
            self.assertEqual(dict(db.execute('SELECT key,value FROM options')), {
                'ModelRatio': '{"model-a":2}', 'SiteName': 'local-site',
            })
            self.assertEqual(db.execute('SELECT key FROM channels').fetchone()[0], 'PRIVATE_KEY')
        self.dump.write_text(self.dump.read_text(encoding='utf-8').replace('1\tt\n', '999\tt\n'), encoding='utf-8')
        with self.assertRaises(ValueError):
            catalog_tool.import_shop(self.dump, self.database)
        with self.open_database() as db:
            self.assertEqual(db.execute('SELECT channel_id FROM abilities').fetchone()[0], 1)

    def test_export_only_metadata_and_copy_escaping(self):
        counts = catalog_tool.export_catalog(self.dump, self.catalog)
        self.assertEqual(counts, {"models": 1, "vendors": 1})
        text = self.catalog.read_text(encoding="utf-8")
        self.assertNotIn("PRIVATE_", text)
        data = json.loads(text)
        self.assertEqual(data["vendors"][0]["description"], "Info\tline\nnext")
        self.assertEqual(data["models"][0]["vendor_name"], "Vendor")
        self.assertNotIn("id", data["models"][0])
        with self.assertRaises(FileExistsError):
            catalog_tool.export_catalog(self.dump, self.catalog)

    def test_import_remaps_vendor_ids_skips_existing_and_preserves_other_tables(self):
        catalog_tool.export_catalog(self.dump, self.catalog)
        self.make_database()
        with self.open_database() as db:
            db.execute("INSERT INTO vendors (id,name,description) VALUES (7,'Vendor','keep custom text')")
        result = catalog_tool.import_catalog(self.catalog, self.database)
        self.assertEqual(result["models_added"], 1)
        self.assertTrue(Path(result["backup"]).is_file())
        with self.open_database() as db:
            self.assertEqual(db.execute("SELECT vendor_id,description FROM models").fetchone(), (7, "A model's description"))
            db.execute("UPDATE models SET description='test customization'")
            for table in ("users", "channels", "options"):
                self.assertEqual(db.execute(f"SELECT count(*) FROM {table}").fetchone()[0], 1)
        second = catalog_tool.import_catalog(self.catalog, self.database)
        self.assertEqual(second["models_added"], 0)
        self.assertEqual(second["models_skipped"], 1)
        with self.open_database() as db:
            self.assertEqual(db.execute("SELECT description FROM models").fetchone()[0], "test customization")
            self.assertEqual(db.execute("SELECT description FROM vendors").fetchone()[0], "keep custom text")
            self.assertEqual(db.execute("SELECT email FROM users").fetchone()[0], "untouched")
            self.assertEqual(db.execute("SELECT key FROM channels").fetchone()[0], "untouched")
            self.assertEqual(db.execute("SELECT value FROM options").fetchone()[0], "untouched")

    def test_import_rolls_back_all_rows_when_an_insert_fails(self):
        catalog_tool.export_catalog(self.dump, self.catalog)
        self.make_database()
        with self.open_database() as db:
            db.execute("CREATE TRIGGER fail_model BEFORE INSERT ON models BEGIN SELECT RAISE(ABORT,'synthetic failure'); END")
        with self.assertRaises(sqlite3.IntegrityError):
            catalog_tool.import_catalog(self.catalog, self.database)
        with self.open_database() as db:
            self.assertEqual(db.execute("SELECT count(*) FROM vendors").fetchone()[0], 0)
            self.assertEqual(db.execute("SELECT count(*) FROM models").fetchone()[0], 0)
            self.assertEqual(db.execute("SELECT count(*) FROM users").fetchone()[0], 1)

    def test_unknown_fields_and_uninitialized_databases_are_rejected(self):
        catalog_tool.export_catalog(self.dump, self.catalog)
        data = json.loads(self.catalog.read_text(encoding="utf-8"))
        data["models"][0]["api_key"] = "must not be accepted"
        self.catalog.write_text(json.dumps(data), encoding="utf-8")
        with self.assertRaises(ValueError):
            catalog_tool.import_catalog(self.catalog, self.database)
        del data["models"][0]["api_key"]
        self.catalog.write_text(json.dumps(data), encoding="utf-8")
        with self.assertRaises(FileNotFoundError):
            catalog_tool.import_catalog(self.catalog, self.database)
        self.assertFalse(self.database.exists())


if __name__ == "__main__":
    unittest.main()
