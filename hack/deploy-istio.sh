#!/usr/bin/env bash
# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked
source .env

ISTIOD_DEPLOYMENT_NAME="istiod"
ISTIO_NAMESPACE="istio-system"

readonly ISTIO_VERSION=${ISTIO_VERSION:-$ENV_ISTIO_VERSION}

function apply_istio_telemetry() {
  kubectl apply -f - <<EOF
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: access-config
  namespace: "$ISTIO_NAMESPACE"
spec:
  accessLogging:
    - providers:
        - name: stdout-json
        - name: kyma-logs
  tracing:
    - providers:
        - name: "kyma-traces"
      randomSamplingPercentage: 100.00
EOF
}

function is_istio_telemetry_crd_available() {
  kubectl get crd telemetries.telemetry.istio.io &> /dev/null
}

function is_istio_telemetry_apply_successful() {
  kubectl get telemetries.telemetry.istio.io access-config -n "$ISTIO_NAMESPACE" &> /dev/null
}

function ensure_istio_telemetry() {
  MAX_ATTEMPTS=10
  DELAY_SECONDS=30

  for ((attempts=1; attempts<=MAX_ATTEMPTS; attempts++)); do
    echo "Attempting to create Istio Telemetry (Attempt $attempts)..."

    if is_istio_telemetry_crd_available; then
      echo "Istio crd available, trying to apply telemetry..."
      apply_istio_telemetry || true
    fi

    if is_istio_telemetry_apply_successful; then
      echo "Istio Telemetry created successfully!"
      return
    else
      echo "Istio Telemetry creation failed. Retrying in $DELAY_SECONDS seconds..."
      sleep $DELAY_SECONDS
    fi
  done

  echo "Maximum attempts reached. Istio Telemetry creation failed!"
  exit 1
}

function apply_peer_authentication() {
  local name=$1
  local namespace=$2
  local mtlsMode=$3

  kubectl apply -f - <<EOF
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: $name
  namespace: $namespace
spec:
  mtls:
    mode: $mtlsMode
EOF
}

function is_peer_authentication_apply_successful() {
  local name=$1
  local namespace=$2

  kubectl get peerauthentications.security.istio.io $name -n $namespace &> /dev/null
}

function ensure_peer_authentication() {
  local name=$1
  local namespace=$2
  local mtlsMode=$3

  MAX_ATTEMPTS=10
  DELAY_SECONDS=30

  for ((attempts=1; attempts<=MAX_ATTEMPTS; attempts++)); do
    echo "Attempting to create Istio Mesh PeerAuthentication (Attempt $attempts)..."
    apply_peer_authentication $name $namespace $mtlsMode || true

    if is_peer_authentication_apply_successful $name $namespace; then
      echo "Istio Mesh PeerAuthentication created successfully!"
      return
    else
      echo "Istio Mesh PeerAuthentication creation failed. Retrying in $DELAY_SECONDS seconds..."
      sleep $DELAY_SECONDS
    fi
  done

  echo "Maximum attempts reached. Istio Mesh PeerAuthentication creation failed!"
  exit 1
}

function check_istiod_is_ready() {
  MAX_ATTEMPTS=10
  DELAY_SECONDS=30

  for ((attempts=1; attempts<=MAX_ATTEMPTS; attempts++)); do
    echo "Checking istiod deployment status"
    check=$(check_istiod_deployment_ready)
    echo "$check"

    if [ "$check" == "ready" ]; then
      echo "Isiod running successfully!"
      return
    else
      kubectl get pods -n "$ISTIO_NAMESPACE"
      echo "Istiod is not ready. Checking again in $DELAY_SECONDS seconds..."
      sleep $DELAY_SECONDS
    fi
  done

  echo "Maximum attempts reached. Telemetry manager is not ready!"
  exit 1
}

function check_istiod_deployment_ready() {
    DESIRED=$(kubectl get deployment "$ISTIOD_DEPLOYMENT_NAME" -n "$ISTIO_NAMESPACE" -o jsonpath='{.spec.replicas}')
    CURRENT=$(kubectl get deployment "$ISTIOD_DEPLOYMENT_NAME"  -n "$ISTIO_NAMESPACE" -o jsonpath='{.status.readyReplicas}')
    if [ "$CURRENT" == "$DESIRED" ]; then
        echo "ready"
    else
        echo "not ready"
    fi
}

function apply_namespace() {
  local namespace=$1
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $namespace
  labels:
    istio-injection: enabled
EOF
}

function wait_for_service_account() {
  local namespace=$1
  local max_attempts=30
  local delay_seconds=2

  echo "Waiting for default service account in namespace $namespace..."
  for ((attempts=1; attempts<=max_attempts; attempts++)); do
    if kubectl get serviceaccount default -n "$namespace" &>/dev/null; then
      echo "Default service account ready in namespace $namespace"
      return 0
    fi
    echo "Attempt $attempts/$max_attempts: Service account not ready yet, waiting $delay_seconds seconds..."
    sleep $delay_seconds
  done

  echo "ERROR: Default service account not created in namespace $namespace after $max_attempts attempts"
  return 1
}

function main() {
  kubectl apply -f "https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-manager.yaml"
  kubectl apply -f "https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-default-cr.yaml"

  check_istiod_is_ready
  ensure_istio_telemetry
  ensure_peer_authentication default "$ISTIO_NAMESPACE" STRICT

  apply_namespace "istio-permissive-mtls"
  wait_for_service_account "istio-permissive-mtls"
  ensure_peer_authentication default istio-permissive-mtls PERMISSIVE
}

main
