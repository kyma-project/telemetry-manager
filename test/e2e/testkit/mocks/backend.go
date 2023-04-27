//go:build e2e

package mocks

import (
	"path/filepath"
)

type Backend struct {
	name             string
	namespace        string
	exportedFilePath string
	signalType       SignalType
}

func NewBackend(name, namespace, exportedFilePath string, signalType SignalType) *Backend {
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

func (b *Backend) ConfigMap(name string) *BackendConfigMap {
	return NewBackendConfigMap(name, b.namespace, b.exportedFilePath, b.signalType)
}

func (b *Backend) Deployment(configMapName string) *BackendDeployment {
	return NewBackendDeployment(b.name, b.namespace, configMapName, filepath.Dir(b.exportedFilePath))
}

func (b *Backend) ExternalService() *ExternalBackendService {
	return NewExternalBackendService(b.name, b.namespace)
}
