#!/usr/bin/env bash

# Test script to diagnose DNS issues in Kubernetes 1.35
# This script tests DNS resolution from within the cluster

set -e

NAMESPACE="${1:-kyma-system}"
SERVICE_NAME="${2:-telemetry-self-monitor}"

echo "=== Testing DNS Resolution in Kubernetes 1.35 ==="
echo "Namespace: $NAMESPACE"
echo "Service: $SERVICE_NAME"
echo ""

# Check if service exists
echo "1. Checking if service exists..."
if ! kubectl get svc -n "$NAMESPACE" "$SERVICE_NAME" &>/dev/null; then
    echo "❌ Service $SERVICE_NAME does not exist in namespace $NAMESPACE"
    echo "Creating a test service..."
    kubectl create service clusterip "$SERVICE_NAME" --tcp=9090:9090 -n "$NAMESPACE" || true
fi

kubectl get svc -n "$NAMESPACE" "$SERVICE_NAME" -o wide
echo ""

# Check endpoints
echo "2. Checking service endpoints..."
kubectl get endpoints -n "$NAMESPACE" "$SERVICE_NAME" 2>/dev/null || echo "No endpoints found"
echo ""

# Test DNS resolution patterns
echo "3. Testing DNS resolution from a pod in the same namespace..."

kubectl run dns-test-$$  -n "$NAMESPACE" --image=busybox:1.36 --restart=Never --rm -i --command -- sh -c "
echo '=== DNS Configuration ==='
cat /etc/resolv.conf
echo ''
echo '=== Testing Resolution Patterns ==='
echo '1. Short name:'
nslookup $SERVICE_NAME 2>&1 | head -10
echo ''
echo '2. service.namespace:'
nslookup $SERVICE_NAME.$NAMESPACE 2>&1 | head -10
echo ''
echo '3. service.namespace.svc:'
nslookup $SERVICE_NAME.$NAMESPACE.svc 2>&1 | head -10
echo ''
echo '4. Full FQDN:'
nslookup $SERVICE_NAME.$NAMESPACE.svc.cluster.local 2>&1 | head -10
" 2>&1

echo ""
echo "4. Testing with dig (if available)..."
kubectl run dig-test-$$ -n "$NAMESPACE" --image=nicolaka/netshoot:latest --restart=Never --rm -i --command -- sh -c "
echo 'Short name:'
dig +short $SERVICE_NAME
echo 'service.namespace:'
dig +short $SERVICE_NAME.$NAMESPACE
echo 'Full FQDN:'
dig +short $SERVICE_NAME.$NAMESPACE.svc.cluster.local
" 2>&1 || echo "dig test failed"

echo ""
echo "5. Checking CoreDNS logs for errors..."
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=20 | grep -i "error\|warn" || echo "No recent errors in CoreDNS"

echo ""
echo "=== Test Complete ==="
