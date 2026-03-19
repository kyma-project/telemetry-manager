#!/usr/bin/env bash
# Reads the live value of sum(fluentbit_input_storage_chunks_down) from the cluster's
# self-monitor Prometheus (same metric used for the buffer-in-use alert).
#
# Prerequisites: kubectl configured for your cluster.
#
# Usage:
#   ./scripts/get-chunks-down-threshold.sh [namespace]
#
# Optional: set NAMESPACE (e.g. export NAMESPACE=kyma-system) or pass as first argument.
# Default namespace: kyma-system

set -e

NAMESPACE="${1:-${NAMESPACE:-kyma-system}}"
SVC="telemetry-self-monitor"
PROM_PORT=9090
QUERY='sum(fluentbit_input_storage_chunks_down{service="telemetry-fluent-bit-metrics"})'

# Ensure we have kubectl
if ! command -v kubectl &>/dev/null; then
  echo "Error: kubectl not found" >&2
  exit 1
fi

# Check service exists
if ! kubectl get "svc/$SVC" -n "$NAMESPACE" &>/dev/null; then
  echo "Error: service $SVC not found in namespace $NAMESPACE" >&2
  echo "Use: $0 <namespace>  or  NAMESPACE=<ns> $0" >&2
  exit 1
fi

# Port-forward in background and wait until it listens
kubectl port-forward -n "$NAMESPACE" "svc/$SVC" "$PROM_PORT:$PROM_PORT" &>/dev/null &
PF_PID=$!
trap 'kill $PF_PID 2>/dev/null || true' EXIT

# Wait for port
for _ in {1..30}; do
  if curl -sS --connect-timeout 1 "http://127.0.0.1:$PROM_PORT/-/healthy" &>/dev/null; then
    break
  fi
  sleep 0.2
done
if ! curl -sS --connect-timeout 1 "http://127.0.0.1:$PROM_PORT/-/healthy" &>/dev/null; then
  echo "Error: Prometheus did not become ready on port $PROM_PORT" >&2
  exit 1
fi

# Query Prometheus API (URL-encode: = { } " and space so the whole query is one param)
ENCODED_QUERY=$(printf '%s' "$QUERY" | sed 's/ /%20/g; s/"/%22/g; s/=/%3D/g; s/{/%7B/g; s/}/%7D/g')
RESP=$(curl -sS --connect-timeout 5 "http://127.0.0.1:$PROM_PORT/api/v1/query?query=$ENCODED_QUERY")

# Parse value: Prometheus returns .data.result[0].value[1] for instant query (value is string)
if command -v jq &>/dev/null; then
  VAL=$(printf '%s' "$RESP" | jq -r '.data.result[0].value[1] // empty')
  if [[ -z "$VAL" ]]; then
    if printf '%s' "$RESP" | jq -e '.error' &>/dev/null; then
      echo "Error: Prometheus query failed: $(printf '%s' "$RESP" | jq -r '.error')" >&2
      exit 1
    fi
  fi
else
  # No jq: extract "value":["<ts>","<val>"] with grep/sed
  if printf '%s' "$RESP" | grep -q '"status":"error"'; then
    echo "Error: Prometheus query failed (install jq for details)" >&2
    exit 1
  fi
  VAL=$(printf '%s' "$RESP" | sed -n 's/.*"value":\[[^,]*, *"\([^"]*\)"\].*/\1/p' | head -1)
fi

if [[ -z "$VAL" ]]; then
  # No result means 0 chunks (no Fluent Bit metrics yet)
  echo "0"
else
  echo "$VAL"
fi
