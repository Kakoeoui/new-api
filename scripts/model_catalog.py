"""Export model metadata from a PostgreSQL dump and merge it into staging SQLite.

The dump is parsed as text, never executed. Only models and their vendors are read.
"""

import argparse
import json
import re
import sqlite3
from contextlib import closing
from datetime import datetime, timezone
from pathlib import Path


VENDOR_FIELDS = ("name", "description", "icon", "status", "created_time", "updated_time")
MODEL_FIELDS = (
    "model_name", "description", "icon", "tags", "endpoints", "status",
    "sync_official", "created_time", "updated_time", "name_rule", "vendor_name",
)
NUMBERS = {"status", "sync_official", "created_time", "updated_time", "name_rule"}
COPY_HEADER = re.compile(r'^COPY public\.([a-z_]+) \(([^)]+)\) FROM stdin;$')
COPY_ESCAPE = re.compile(r"\\([0-7]{1,3}|x[0-9a-fA-F]{1,2}|.)")


def decode_copy(value):
    if value == r"\N":
        return None

    def unescape(match):
        char = match.group(1)
        if char[0] in "01234567":
            return chr(int(char, 8))
        if char.startswith("x") and len(char) > 1:
            return chr(int(char[1:], 16))
        return {"b": "\b", "f": "\f", "n": "\n", "r": "\r", "t": "\t", "v": "\v"}.get(char, char)

    return COPY_ESCAPE.sub(unescape, value)


def read_dump(dump_path, table_names=("models", "vendors")):
    tables = {name: [] for name in table_names}
    table = None
    columns = []
    with Path(dump_path).open(encoding="utf-8") as source:
        for line in source:
            line = line.rstrip("\r\n")
            if table:
                if line == r"\.":
                    table = None
                    continue
                values = line.split("\t")
                if len(values) != len(columns):
                    raise ValueError(f"Invalid COPY row in {table}; no data exported")
                row = dict(zip(columns, map(decode_copy, values)))
                if row.get("deleted_at") in (None, ""):
                    tables[table].append(row)
                continue
            match = COPY_HEADER.fullmatch(line)
            if match and match[1] in tables:
                table = match[1]
                columns = [column.strip().strip('"') for column in match[2].split(",")]
                if len(set(columns)) != len(columns):
                    raise ValueError("Duplicate COPY columns")
    if table:
        raise ValueError("Incomplete COPY block")
    if not tables["models"]:
        raise ValueError("No non-deleted models found in this backup")
    return tables


SHOP_TABLES = ("vendors", "models", "channels", "abilities", "options")
PRICE_OPTIONS = {
    "ModelRatio", "ModelPrice", "CompletionRatio", "CacheRatio", "CreateCacheRatio",
    "ImageRatio", "AudioRatio", "AudioCompletionRatio", "GroupRatio", "GroupGroupRatio",
    "UserUsableGroups", "QuotaPerUnit",
    "general_setting.quota_display_type", "general_setting.custom_currency_symbol",
    "general_setting.custom_currency_exchange_rate", "general_setting.usd_exchange_rate",
    "group_ratio_setting.group_ratio", "group_ratio_setting.group_group_ratio",
    "group_ratio_setting.group_special_usable_group", "billing_setting.model_pricing",
}


def import_shop(dump_path, database_path):
    """Replace local model/channel settings from a dump, preserving user data."""
    tables = read_dump(dump_path, SHOP_TABLES)
    tables["options"] = [row for row in tables["options"] if row.get("key") in PRICE_OPTIONS]
    if not tables["channels"] or not tables["abilities"]:
        raise ValueError("Backup has no channels/abilities; local database was not changed")
    database_path = Path(database_path).resolve(strict=True)
    with database_path.open("rb") as source:
        if source.read(16) != b"SQLite format 3\x00":
            raise ValueError("Target must be the initialized local SQLite database")
    with closing(sqlite3.connect(database_path.as_uri() + "?mode=rw", uri=True)) as db:
        schemas = {}
        for table in SHOP_TABLES:
            schemas[table] = {row[1]: row[2].upper() for row in db.execute(f'PRAGMA table_info("{table}")')}
            if not schemas[table]:
                raise ValueError(f"Missing local table: {table}")
            for row in tables[table]:
                if not set(row) <= schemas[table].keys():
                    raise ValueError(f"Incompatible backup columns in {table}; no changes made")
                # PostgreSQL COPY booleans are text; SQLite/GORM expects 0/1.
                for column, value in row.items():
                    if ("BOOL" in schemas[table][column] or (table == "abilities" and column == "enabled")) and value is not None:
                        if value not in ("t", "f", "true", "false", "0", "1"):
                            raise ValueError(f"Invalid boolean in {table}")
                        row[column] = int(value in ("t", "true", "1"))
        channel_ids = {row["id"] for row in tables["channels"]}
        if any(row["channel_id"] not in channel_ids for row in tables["abilities"]):
            raise ValueError("Backup has abilities referencing missing channels")
        stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
        backup_path = database_path.with_name(database_path.name + f".before-shop-{stamp}.db")
        with backup_path.open("xb"):
            pass
        with closing(sqlite3.connect(backup_path)) as backup:
            db.backup(backup)
        with db:
            # Replace this configuration as a set so channel/model IDs stay aligned.
            for table in ("abilities", "channels", "models", "vendors"):
                db.execute(f'DELETE FROM "{table}"')
            for table in SHOP_TABLES:
                for row in tables[table]:
                    if table == "options":
                        db.execute('DELETE FROM options WHERE "key"=?', (row["key"],))
                    columns = ",".join('"' + name + '"' for name in row)
                    placeholders = ",".join("?" for _ in row)
                    db.execute(f'INSERT INTO "{table}" ({columns}) VALUES ({placeholders})', tuple(row.values()))
    return {**{table: len(rows) for table, rows in tables.items()}, "backup": str(backup_path)}


def project_row(row, fields):
    projected = {}
    for field in fields:
        value = row.get(field)
        projected[field] = int(value or 0) if field in NUMBERS else (value or "")
    return projected


def export_catalog(dump_path, output_path):
    tables = read_dump(dump_path)
    vendor_ids = {int(row["id"]): row for row in tables["vendors"]}
    used_vendors = {}
    models = []
    for row in tables["models"]:
        item = project_row(row, MODEL_FIELDS)
        vendor_id = int(row.get("vendor_id") or 0)
        if vendor_id:
            if vendor_id not in vendor_ids:
                raise ValueError("A model references a missing/deleted vendor")
            vendor = project_row(vendor_ids[vendor_id], VENDOR_FIELDS)
            used_vendors[vendor["name"]] = vendor
            item["vendor_name"] = vendor["name"]
        models.append(item)
    catalog = {"version": 1, "vendors": list(used_vendors.values()), "models": models}
    validate_catalog(catalog)
    # Never overwrite an earlier export silently.
    with Path(output_path).open("x", encoding="utf-8") as target:
        json.dump(catalog, target, ensure_ascii=False, indent=2)
        target.write("\n")
    return {"models": len(models), "vendors": len(used_vendors)}


def validate_catalog(catalog):
    if set(catalog) != {"version", "models", "vendors"} or catalog["version"] != 1:
        raise ValueError("Unsupported catalog format")
    for table, fields, name_key in (("vendors", VENDOR_FIELDS, "name"), ("models", MODEL_FIELDS, "model_name")):
        if not isinstance(catalog[table], list):
            raise ValueError("Catalog rows must be arrays")
        names = set()
        for row in catalog[table]:
            if not isinstance(row, dict) or set(row) != set(fields):
                raise ValueError(f"Unexpected fields in {table}")
            for field, value in row.items():
                if field in NUMBERS:
                    if type(value) is not int or not 0 <= value < 2**63:
                        raise ValueError("Invalid numeric metadata")
                elif not isinstance(value, str) or "\x00" in value:
                    raise ValueError("Invalid text metadata")
            name = row[name_key]
            if not name or len(name) > 128 or name in names:
                raise ValueError(f"Empty, oversized or duplicate name in {table}")
            names.add(name)
    if not catalog["models"]:
        raise ValueError("Catalog contains no models")
    vendor_names = {row["name"] for row in catalog["vendors"]}
    for row in catalog["models"]:
        if row["name_rule"] not in (0, 1, 2, 3):
            raise ValueError("Invalid model matching rule")
        if row["vendor_name"] and row["vendor_name"] not in vendor_names:
            raise ValueError("Missing vendor in catalog")


def import_catalog(catalog_path, database_path):
    catalog = json.loads(Path(catalog_path).read_text(encoding="utf-8"))
    validate_catalog(catalog)
    database_path = Path(database_path).resolve(strict=True)
    with database_path.open("rb") as source:
        if source.read(16) != b"SQLite format 3\x00":
            raise ValueError("Target is not an initialized SQLite database")
    counts = {"models_added": 0, "models_skipped": 0, "vendors_added": 0}
    with closing(sqlite3.connect(database_path.as_uri() + "?mode=rw", uri=True)) as db:
        for table, required in (("models", set(MODEL_FIELDS) - {"vendor_name"} | {"vendor_id", "id", "deleted_at"}),
                                ("vendors", set(VENDOR_FIELDS) | {"id", "deleted_at"})):
            actual = {row[1] for row in db.execute(f"PRAGMA table_info({table})")}
            if not required <= actual:
                raise ValueError("Target schema is missing model tables/columns; start staging first")

        stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
        backup_path = database_path.with_name(database_path.name + f".before-models-{stamp}.db")
        with backup_path.open("xb"):
            pass
        with closing(sqlite3.connect(backup_path)) as backup:
            db.backup(backup)

        with db:
            vendor_ids = {}
            for row in catalog["vendors"]:
                found = db.execute("SELECT id FROM vendors WHERE name=? AND deleted_at IS NULL ORDER BY id LIMIT 1", (row["name"],)).fetchone()
                if found:
                    vendor_ids[row["name"]] = found[0]
                else:
                    columns = ",".join(VENDOR_FIELDS)
                    placeholders = ",".join("?" for _ in VENDOR_FIELDS)
                    cursor = db.execute(f"INSERT INTO vendors ({columns}) VALUES ({placeholders})", tuple(row[field] for field in VENDOR_FIELDS))
                    vendor_ids[row["name"]] = cursor.lastrowid
                    counts["vendors_added"] += 1
            for row in catalog["models"]:
                if db.execute("SELECT 1 FROM models WHERE model_name=? AND deleted_at IS NULL LIMIT 1", (row["model_name"],)).fetchone():
                    counts["models_skipped"] += 1
                    continue
                columns = tuple(field for field in MODEL_FIELDS if field != "vendor_name") + ("vendor_id",)
                values = tuple(row[field] for field in columns[:-1]) + (vendor_ids.get(row["vendor_name"], 0),)
                placeholders = ",".join("?" for _ in columns)
                db.execute(f"INSERT INTO models ({','.join(columns)}) VALUES ({placeholders})", values)
                counts["models_added"] += 1
    return {**counts, "backup": str(backup_path)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    export = commands.add_parser("export")
    export.add_argument("--dump", required=True)
    export.add_argument("--output", required=True)
    load = commands.add_parser("import")
    load.add_argument("--catalog", required=True)
    load.add_argument("--database", required=True)
    shop = commands.add_parser("import-shop")
    shop.add_argument("--dump", required=True)
    shop.add_argument("--database", required=True)
    args = parser.parse_args()
    if args.command == "export":
        result = export_catalog(args.dump, args.output)
    elif args.command == "import-shop":
        result = import_shop(args.dump, args.database)
    else:
        result = import_catalog(args.catalog, args.database)
    print(json.dumps(result))


if __name__ == "__main__":
    main()
