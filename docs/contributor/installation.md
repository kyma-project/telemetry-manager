# Installation

In the following possible ways of installation are outlined. More details on the available make targets and the prerequisites can be found at [Development](./development.md).

## Install Telemetry Manager from sources

```sh
make install
make run
```

## Install Telemetry Manager in your cluster from sources

```bash
export IMG=<my container repo>
make docker-build
make docker-push
kubectl create ns kyma-system
make deploy
```

## Install Telemetry Manager in your cluster from latest release

```
kubectl create ns kyma-system
kubectl apply -f https://github.com/kyma-project/telemetry-manager/releases/latest/download/rendered.yaml
```

## Install Telemetry Manager in your cluster from latest release using the lifecycle manager

Install the lifecycle-manager

```shell
make kyma
kyma alpha deploy
```

Install the ModuleTemplate and activate the component
```shell
kubectl apply -f https://github.com/kyma-project/btp-manager/releases/latest/download/template.yaml
kyma alpha enable module telemetry
```
