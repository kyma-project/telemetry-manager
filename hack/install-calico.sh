#!/usr/bin/env bash
# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # needs to be set if we want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

readonly CALICO_VERSION=${CALICO_VERSION:-"v3.29.0"}
readonly TIGERA_OPERATOR_URL="https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml"
readonly CUSTOM_RESOURCES_URL="https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/custom-resources.yaml"

readonly MAX_ATTEMPTS=30
readonly DELAY_SECONDS=10

function log() {
  echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

function apply_tigera_operator() {
  log "Applying Tigera operator..."
  kubectl create -f "$TIGERA_OPERATOR_URL"
}

function apply_custom_resources() {
  log "Applying Calico custom resources..."
  kubectl create -f "$CUSTOM_RESOURCES_URL"
}

function is_tigera_status_available() {
  local component=$1
  kubectl get tigerastatus "$component" &> /dev/null
}

function is_tigera_status_ready() {
  local component=$1
  local condition=$(kubectl get tigerastatus "$component" -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null || echo "False")
  [[ "$condition" == "True" ]]
}

function wait_for_tigera_status() {
  local component=$1
  log "Waiting for TigeraStatus '$component' to be available..."

  for ((attempts=1; attempts<=MAX_ATTEMPTS; attempts++)); do
    log "Checking TigeraStatus '$component' (Attempt $attempts/$MAX_ATTEMPTS)..."

    if is_tigera_status_available "$component"; then
      if is_tigera_status_ready "$component"; then
        log "TigeraStatus '$component' is ready!"
        return 0
      else
        local status=$(kubectl get tigerastatus "$component" -o jsonpath='{.status.conditions[?(@.type=="Available")]}' 2>/dev/null || echo "Not found")
        log "TigeraStatus '$component' not ready yet. Status: $status"
      fi
    else
      log "TigeraStatus '$component' not found yet..."
    fi

    if [[ $attempts -lt $MAX_ATTEMPTS ]]; then
      log "Waiting $DELAY_SECONDS seconds before next check..."
      sleep $DELAY_SECONDS
    fi
  done

  log "ERROR: Timeout waiting for TigeraStatus '$component' to be ready"
  return 1
}

function verify_calico_installation() {
  log "Verifying Calico installation..."

  # Check if we can create a simple NetworkPolicy
  kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: calico-test-policy
  namespace: default
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
EOF

  if kubectl get networkpolicy calico-test-policy -n default &> /dev/null; then
    log "NetworkPolicy creation successful - Calico is working!"
    kubectl delete networkpolicy calico-test-policy -n default
    return 0
  else
    log "ERROR: Failed to create test NetworkPolicy"
    return 1
  fi
}

function cleanup_on_error() {
  log "Cleaning up due to error..."
  kubectl delete -f "$CUSTOM_RESOURCES_URL" --ignore-not-found=true || true
  kubectl delete -f "$TIGERA_OPERATOR_URL" --ignore-not-found=true || true
}

function main() {
  log "Starting Calico CNI installation (version: $CALICO_VERSION)"

  # Set up error handling
  trap cleanup_on_error ERR

  # Check if kubectl is available
  if ! command -v kubectl &> /dev/null; then
    log "ERROR: kubectl is not available"
    exit 1
  fi

  # Check if cluster is accessible
  if ! kubectl cluster-info &> /dev/null; then
    log "ERROR: Cannot connect to Kubernetes cluster"
    exit 1
  fi

  # Check if Calico is already installed
  if kubectl get tigerastatus calico &> /dev/null; then
    log "Calico appears to be already installed. Checking status..."
    if is_tigera_status_ready calico; then
      log "Calico is already installed and ready!"
      exit 0
    else
      log "Calico is installed but not ready. Continuing with status checks..."
    fi
  else
    # Install Tigera operator
    apply_tigera_operator

    # Install Calico custom resources
    apply_custom_resources
  fi

  # Wait for components to be ready
  wait_for_tigera_status apiserver
  wait_for_tigera_status calico
  wait_for_tigera_status ippools

  # Verify installation works
  verify_calico_installation

  log "Calico CNI installation completed successfully!"
  log "You can now create NetworkPolicies in your cluster."
}

main "$@"
