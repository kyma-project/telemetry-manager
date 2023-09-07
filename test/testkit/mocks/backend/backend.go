package backend

import (
	"path/filepath"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/fluentd"
)

type Backend struct {
	name             string
	namespace        string
	exportedFilePath string
	signalType       SignalType
	withTLS          bool
	certs            testkit.TLSCerts
}

func New(name, namespace, exportedFilePath string, signalType SignalType, withTLS bool, certs testkit.TLSCerts) *Backend {
	return &Backend{
		name:             name,
		namespace:        namespace,
		exportedFilePath: exportedFilePath,
		signalType:       signalType,
		withTLS:          withTLS,
		certs:            certs,
	}
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) ConfigMap(name string) *ConfigMap {
	return NewConfigMap(name, b.namespace, b.exportedFilePath, b.signalType, b.withTLS, b.certs)
}

func (b *Backend) FluentdConfigMap(name string) *fluentd.ConfigMap {
	return fluentd.NewConfigMap(name, b.namespace)
}

func (b *Backend) Deployment(configMapName string) *Deployment {
	return NewDeployment(b.name, b.namespace, configMapName, filepath.Dir(b.exportedFilePath), b.signalType)
}

func (b *Backend) ExternalService() *ExternalService {
	return NewExternalService(b.name, b.namespace)
}
