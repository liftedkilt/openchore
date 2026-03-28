#!/usr/bin/env bash
set -euo pipefail

WIPE=false

for arg in "$@"; do
  case "$arg" in
    --wipe) WIPE=true ;;
    -h|--help)
      echo "Usage: ./redeploy.sh [--wipe]"
      echo ""
      echo "  --wipe  Remove volumes (database) before redeploying"
      exit 0
      ;;
    *)
      echo "Unknown option: $arg"
      echo "Usage: ./redeploy.sh [--wipe]"
      exit 1
      ;;
  esac
done

cd "$(dirname "$0")"

if [ "$WIPE" = true ]; then
  echo "==> Stopping containers and removing volumes..."
  docker compose down -v
else
  echo "==> Stopping containers..."
  docker compose down
fi

echo "==> Pulling latest code..."
git pull

echo "==> Pulling latest images..."
docker compose pull

echo "==> Starting containers..."
docker compose up -d

echo "==> Done."
