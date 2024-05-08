#!/usr/bin/env bash
source .env

readonly ISTIO_VERSION=${ISTIO_VERSION:-$ENV_ISTIO_VERSION}

function apply_istio_telemetry() {
  kubectl apply -f - <<EOF
apiVersion: telemetry.istio.io/v1alpha1
kind: Telemetry
metadata:
  name: access-config
  namespace: istio-system
spec:
  accessLogging:
    - providers:
        - name: stdout-json
  tracing:
    - providers:
        - name: "kyma-traces"
      randomSamplingPercentage: 100.00
EOF
}

function is_istio_telemetry_apply_successful() {
  kubectl get telemetries.telemetry.istio.io access-config -n istio-system &> /dev/null
}

function ensure_istio_telemetry() {
  MAX_ATTEMPTS=10
  DELAY_SECONDS=30

  for ((attempts=1; attempts<=MAX_ATTEMPTS; attempts++)); do
    echo "Attempting to create Istio Telemetry (Attempt $attempts)..."
    apply_istio_telemetry

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
apiVersion: security.istio.io/v1beta1
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
    apply_peer_authentication $name $namespace $mtlsMode

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

function main() {
  kubectl apply -f "https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-manager.yaml"
  kubectl apply -f "https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-default-cr.yaml"
  ensure_istio_telemetry
  ensure_peer_authentication default istio-system STRICT

  kubectl create namespace istio-permissive-mtls
  kubectl label namespace istio-permissive-mtls istio-injection=enabled --overwrite
  ensure_peer_authentication default istio-permissive-mtls PERMISSIVE
}

main
