#!/bin/bash

echo "=========================================="
echo "E2E Test Diagnostics"
echo "=========================================="
echo ""

echo "=== Telemetry Manager Status ==="
kubectl get deployment -n kyma-system | grep telemetry
echo ""

echo "=== Telemetry Manager Pods ==="
kubectl get pods -n kyma-system | grep telemetry
echo ""

echo "=== LogPipelines ==="
kubectl get logpipelines -A
echo ""

echo "=== Log Gateway ==="
kubectl get deployment -n kyma-system | grep log-gateway
kubectl get pods -n kyma-system | grep log-gateway
echo ""

echo "=== Log Agent (DaemonSet) ==="
kubectl get daemonset -n kyma-system | grep log-agent
kubectl get pods -n kyma-system | grep log-agent
echo ""

echo "=== Test Namespaces (should have backend/gen namespaces) ==="
kubectl get namespaces | grep -E "backend|gen|agent"
echo ""

echo "=== Recent Manager Logs (last 20 lines) ==="
kubectl logs -n kyma-system deployment/telemetry-manager --tail=20
echo ""

echo "=== Check for any errors in manager ==="
kubectl logs -n kyma-system deployment/telemetry-manager --tail=100 | grep -i error || echo "No errors found"
echo ""

echo "=== Telemetry CR Status ==="
kubectl get telemetry default -o yaml | grep -A20 "status:" || echo "No Telemetry CR found"
