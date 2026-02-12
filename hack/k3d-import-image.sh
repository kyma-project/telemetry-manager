#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?image required}"
CLUSTER="${2:-kyma}"
MAX_RETRIES="${3:-20}"
K3D_BIN="${K3D:?K3D env var required}"
NODE="k3d-${CLUSTER}-server-0"

wait_for_containerd() {
  echo "Waiting for containerd in $CLUSTER..."
  for _ in {1..30}; do
    if docker exec "$NODE" ctr images ls >/dev/null 2>&1; then
      echo "containerd is ready"
      return 0
    fi
    sleep 2
  done
  echo "containerd not ready in time"
  return 1
}

ensure_image_complete() {
  echo "Ensuring image layers are fully materialized..."
  docker image inspect "$IMAGE" >/dev/null 2>&1
  docker save "$IMAGE" | docker load >/dev/null
}

k3d_import() {
  local output
  output=$("$K3D_BIN" image import "$IMAGE" -c "$CLUSTER" 2>&1 || true)
  echo "$output"
  if echo "$output" | grep -q "ERRO\|error\|failed"; then
    return 1
  fi
  return 0
}

ctr_import() {
  echo "Falling back to direct ctr import..."
  docker save "$IMAGE" | docker exec -i "$NODE" ctr -n k8s.io images import -
  docker exec "$NODE" ctr -n k8s.io images ls | grep -q "$IMAGE"
}

wait_for_containerd
ensure_image_complete

for ((i=1; i<=MAX_RETRIES; i++)); do
  echo "Import attempt $i/$MAX_RETRIES"
  if k3d_import; then
    echo "Image imported successfully"
    exit 0
  fi

  if ctr_import; then
    echo "Image imported successfully via ctr"
    exit 0
  fi

  if [[ $i -eq $MAX_RETRIES ]]; then
    echo "Failed to import image after $MAX_RETRIES attempts"
    exit 1
  fi
  echo "Retrying in 5s..."
  sleep 5
done