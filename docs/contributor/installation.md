# Installation

You can choose among several ways of installation. For more details on the available make targets and the prerequisites, see [Development](./development.md).

## Prerequisites

- See prerequisites for running the make targets at the [Development section](./development.md).
- You have a Kubecontext pointing to an existing Kubernetes cluster.

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

## Install Telemetry Manager in your cluster from latest release using the Lifecycle manager

1. Install the Lifecycle manager:

```shell
make kyma
kyma alpha deploy
```

2. Install the ModuleTemplate and activate the component:
```shell
kubectl apply -f https://github.com/kyma-project/telemetry-manager/releases/latest/download/template.yaml
kyma alpha enable module telemetry --channel fast
```

3. Verify that a `telemetry-operator` Pod starts up in the `kyma-system` Namespace.
