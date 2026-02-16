#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # must be set if you want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

IMAGE="${1:?image required}"

CLUSTER="kyma"
NODE="k3d-${CLUSTER}-server-0"
MAX_RETRIES=10


ctr_import() {
  docker save "$IMAGE" | docker exec -i "$NODE" ctr -n k8s.io images import -
  docker exec "$NODE" ctr -n k8s.io images ls | grep -q "$IMAGE"
}

for ((i=1; i<=MAX_RETRIES; i++)); do
  echo "Import attempt $i/$MAX_RETRIES"

  if ctr_import; then
    echo "Image imported successfully"
    exit 0
  fi

  if [[ $i -eq $MAX_RETRIES ]]; then
    echo "Failed to import image after $MAX_RETRIES attempts"
    exit 1
  fi

  echo "Retrying in 5s..."
  sleep 5
done
