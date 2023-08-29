package backend

import (
	"path/filepath"

	fluentD "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/fluentd"
)

type Backend struct {
	name             string
	namespace        string
	exportedFilePath string
	signalType       SignalType
}

func New(name, namespace, exportedFilePath string, signalType SignalType) *Backend {
	return &Backend{
		name:             name,
		namespace:        namespace,
		exportedFilePath: exportedFilePath,
		signalType:       signalType,
	}
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) ConfigMap(name string) *ConfigMap {
	return NewConfigMap(name, b.namespace, b.exportedFilePath, b.signalType)
}

func (b *Backend) FluentDConfigMap(name string) *fluentD.ConfigMap {
	return fluentD.NewConfigMap(name, b.namespace)
}

func (b *Backend) Deployment(configMapName string) *Deployment {
	return NewDeployment(b.name, b.namespace, configMapName, filepath.Dir(b.exportedFilePath), b.signalType)
}

func (b *Backend) ExternalService() *ExternalService {
	return NewExternalService(b.name, b.namespace)
}
