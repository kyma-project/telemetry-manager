#!/usr/bin/env sh

bin/k3d registry create kyma-registry --port 5001
bin/k3d cluster create kyma --no-lb --registry-use kyma-registry:5001 --k3s-arg --disable=traefik@server:0 --image rancher/k3s:v$K8S_VERSION-k3s1

IMG=localhost:5001/telemetry-manager:latest
export IMG

make docker-build
make docker-push
make install

IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
