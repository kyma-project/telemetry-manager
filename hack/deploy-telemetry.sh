#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked
TELEMETRY_MANAGER_DEPLOYMENT_NAME="telemetry-manager"
TELEMETRY_MANAGER_NAMESPACE="kyma-system"

# shellcheck disable=SC2112
function check_deployment_ready() {
    DESIRED=$(kubectl get deployment "$TELEMETRY_MANAGER_DEPLOYMENT_NAME" -n "$TELEMETRY_MANAGER_NAMESPACE" -o jsonpath='{.spec.replicas}')
    CURRENT=$(kubectl get deployment "$TELEMETRY_MANAGER_DEPLOYMENT_NAME"  -n "$TELEMETRY_MANAGER_NAMESPACE" -o jsonpath='{.status.readyReplicas}')
    if [ "$CURRENT" == "$DESIRED" ]; then
        echo "ready"
    else
        echo "not ready"
    fi
}

# shellcheck disable=SC2112
function check_telemetry_manager_is_ready() {
  MAX_ATTEMPTS=10
  DELAY_SECONDS=30

  for ((attempts=1; attempts<=MAX_ATTEMPTS; attempts++)); do
    echo "Checking deployment status"
    check=$(check_deployment_ready)
    echo "$check"

    if [ "$check" == "ready" ]; then
      echo "Telemetry manager running successfully!"
      return
    else
      kubectl get pods -n kyma-system
      echo "Telemetry manager is not ready. Checking again in $DELAY_SECONDS seconds..."
      sleep $DELAY_SECONDS
    fi
  done

  echo "Maximum attempts reached. Telemetry manager is not ready!"
  exit 1
}


function main() {
  (make deploy && \
      check_telemetry_manager_is_ready)
}

main
