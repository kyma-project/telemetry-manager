#!/usr/bin/env sh

bin/k3d registry create kyma-registry --port 5001
bin/k3d cluster create kyma --registry-use kyma-registry:5001 --image rancher/k3s:v$K8S_VERSION-k3s1 \
 --port "9090:30090@server:0" \
 --port "8888:30088@server:0" \
 --port "4317:30017@server:0" \
 --port "4318:30018@server:0"

IMG=localhost:5001/telemetry-manager:latest
export IMG

make docker-build
make docker-push
make install

IMG=k3d-kyma-registry:5000/telemetry-manager:latest make deploy
