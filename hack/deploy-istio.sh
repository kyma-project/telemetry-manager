#!/usr/bin/env bash

function apply_istio_telemetry() {
  cat <<EOF | kubectl apply -f -
apiVersion: telemetry.istio.io/v1alpha1
kind: Telemetry
metadata:
  name: access-config
  namespace: istio-system
spec:
  accessLogging:
    - providers:
        - name: stdout-json
EOF
}

function is_istio_telemetry_apply_successful() {
  kubectl get telemetries.telemetry.istio.io access-config -n istio-system &> /dev/null
  return $?
}

function ensure_istio_telemetry() {
    MAX_ATTEMPTS=10
    DELAY_SECONDS=30

    # Loop until kubectl apply is successful or maximum attempts are reached
    attempts=1
    while [ $attempts -le $MAX_ATTEMPTS ]; do
        echo "Attempting create Istio Telemetry (Attempt $attempts)..."
        apply_istio_telemetry

        if is_istio_telemetry_apply_successful; then
            echo "Istio Telemetry created successfully!"
            return
        else
            echo "Istio Telemetry creation failed. Retrying in $DELAY_SECONDS seconds..."
            sleep $DELAY_SECONDS
            ((attempts++))
        fi
    done

    echo "Maximum attempts reached. Istio Telemetry creation failed!"
    exit 1
}

# TODO: Remove the creation of PeerAuthentication after it's implemented by the Istio Manager
function apply_mesh_peer_authentication() {
  cat <<EOF | kubectl apply -f -
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
 name: default
 namespace: istio-system
spec:
 mtls:
  mode: STRICT
EOF
}

function is_mesh_peer_authentication_apply_successful() {
  kubectl get peerauthentications.security.istio.io default -n istio-system &> /dev/null
  return $?
}

function ensure_mesh_peer_authentication() {
    MAX_ATTEMPTS=10
    DELAY_SECONDS=30

    # Loop until kubectl apply is successful or maximum attempts are reached
    attempts=1
    while [ $attempts -le $MAX_ATTEMPTS ]; do
        echo "Attempting create Istio Mesh PeerAuthentication (Attempt $attempts)..."
        apply_mesh_peer_authentication

        if is_mesh_peer_authentication_apply_successful; then
            echo "Istio Mesh PeerAuthentication created successfully!"
            return
        else
            echo "Istio Mesh PeerAuthentication creation failed. Retrying in $DELAY_SECONDS seconds..."
            sleep $DELAY_SECONDS
            ((attempts++))
        fi
    done

    echo "Maximum attempts reached. Istio Mesh PeerAuthentication creation failed!"
    exit 1
}

function main() {
  kubectl apply -f https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-manager.yaml
  kubectl apply -f https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-default-cr.yaml
  ensure_istio_telemetry
  ensure_mesh_peer_authentication
}

main

