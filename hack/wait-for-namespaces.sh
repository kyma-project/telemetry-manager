#!/usr/bin/env bash

CURRENT_COMMIT=$(git rev-parse --abbrev-ref HEAD)
TAG_LIST=$(git tag --sort=-creatordate)
LATEST_TAG=${TAG_LIST[0]}

get_test_namespaces() {
  kubectl get namespaces --no-headers=true | awk '{print $1}' | grep -vE "(kube-system|kube-public|kube-node-lease|default|kyma-system)"
}

while true; do
  namespaces=$(get_test_namespaces)

  if [ -z "$namespaces" ]; then
    echo "All test namespaces have terminated."
    break
  else
    echo "Waiting for test namespaces to terminate: $namespaces"
    sleep 10
  fi
done
