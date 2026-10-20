#!/usr/bin/env bash
set -euo pipefail

# One-time data import utility, not another application launcher.
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
catalog="${1:-$HOME/rvl/private/shop/model-catalog.json}"
if [[ ! -f "$catalog" ]]; then
  echo 'Missing model-catalog.json. Export it from the backup and transfer it first.' >&2
  exit 1
fi
catalog="$(cd -- "$(dirname -- "$catalog")" && pwd)/$(basename -- "$catalog")"

image="$(docker container inspect --format '{{.Config.Image}}' new-api-staging)"
volume="$(docker container inspect --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}' new-api-staging)"
if [[ "$image" != new-api:staging || "$volume" != new-api-staging-data ]]; then
  echo 'Refusing import: target is not the isolated staging backend/volume.' >&2
  exit 1
fi

# Pull before pausing the test backend, so a download failure leaves it running.
docker pull python:3.12-alpine
running="$(docker container inspect --format '{{.State.Running}}' new-api-staging)"
restart_staging() {
  if [[ "$running" == true ]]; then
    docker start new-api-staging >/dev/null || echo 'Could not restart staging; run docker start new-api-staging.' >&2
  fi
}
if [[ "$running" == true ]]; then
  docker stop new-api-staging >/dev/null
fi
trap restart_staging EXIT

docker run --rm --network none --user 0 \
  --mount type=volume,source=new-api-staging-data,target=/data \
  --mount "type=bind,source=$catalog,target=/input/catalog.json,readonly" \
  --mount "type=bind,source=$script_dir/model_catalog.py,target=/tool/model_catalog.py,readonly" \
  python:3.12-alpine python /tool/model_catalog.py import \
  --catalog /input/catalog.json --database /data/one-api.db

echo 'Model metadata imported. Existing names were skipped; customer data/channels/keys were not imported.'
