#!/usr/bin/env bash
set -euo pipefail

WIPE=false
PROFILE=""

for arg in "$@"; do
  case "$arg" in
    --wipe) WIPE=true ;;
    --ai) PROFILE="--profile ai" ;;
    -h|--help)
      echo "Usage: ./redeploy.sh [--wipe] [--ai]"
      echo ""
      echo "  --wipe  Remove volumes (database) before redeploying"
      echo "  --ai    Include AI sidecars (LiteRT + Kokoro TTS)"
      exit 0
      ;;
    *)
      echo "Unknown option: $arg"
      echo "Usage: ./redeploy.sh [--wipe] [--ai]"
      exit 1
      ;;
  esac
done

cd "$(dirname "$0")"

echo "==> Pulling latest code..."
git pull

echo "==> Pulling latest images (before stopping — minimizes downtime)..."
docker compose $PROFILE pull

if [ "$WIPE" = true ]; then
  echo "==> Stopping containers and removing volumes..."
  docker compose $PROFILE down -v
else
  echo "==> Stopping containers..."
  docker compose $PROFILE down
fi

echo "==> Starting containers..."
docker compose $PROFILE up -d

echo "==> Waiting for API health check..."
timeout 60 sh -c 'until docker compose ps api --format "{{.Health}}" 2>/dev/null | grep -q healthy; do sleep 2; done' \
  && echo "==> API is healthy." \
  || echo "==> Warning: API health check timed out (may still be starting)."

echo "==> Done."
