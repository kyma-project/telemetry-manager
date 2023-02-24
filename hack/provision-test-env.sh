#!/usr/bin/env sh

kyma provision k3d --ci

REGISTRY_PORT=$(k3d registry list k3d-kyma-registry -ojson | jq '.[0].portMappings."5000/tcp"[0].HostPort')
IMG=localhost:$REGISTRY_PORT/telemetry-manager:latest
export IMG

make docker-build
make docker-push
make install

IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
