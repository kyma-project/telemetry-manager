# Telemetry Self Monitor Docker Image

The container image is built based on the latest [Prometheus LTS image](https://prometheus.io/docs/introduction/release-cycle/).

Additionally, there is a [plugins.yml](./plugins.yml) file, which contains the list of the required plugins. 

## Build locally

To build the image locally with the versions taken from the `envs` file, run:
```
docker build -t telemetry-self-monitor:local --build-arg PROMETHEUS_VERSION=XXX --build-arg ALPINE_VERSION=XXX .
```
