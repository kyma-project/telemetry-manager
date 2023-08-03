#!/usr/bin/env sh

kubectl apply -f https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-manager.yaml
kubectl apply -f https://github.com/kyma-project/istio/releases/download/$ISTIO_VERSION/istio-default-cr.yaml
kubectl create namespace sample
kubectl label namespace sample istio-injection=enabled
kubectl apply -f ./test/integration/resources/istio-access-logs.yaml

