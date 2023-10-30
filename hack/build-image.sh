#!/usr/bin/env sh

IMG=localhost:5001/telemetry-manager:latest
export IMG

make docker-build
make docker-push
