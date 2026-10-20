#!/bin/sh
(
set -eu

# This runs only for a new PostgreSQL volume. Keep the source dump read-only.
# A subshell also isolates shell options when the image sources a non-executable file.
if ! psql -X --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set ON_ERROR_STOP=1 --single-transaction --quiet \
  --file /restore/customer.sql > /tmp/customer-restore.log 2>&1; then
  echo 'Customer restore failed. The app will stay stopped. Details: /tmp/customer-restore.log' >&2
  exit 1
fi

touch /var/lib/postgresql/data/.customer-restore-complete
echo 'Customer database restored. Future starts will keep the existing database.'
)
