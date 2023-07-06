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

func NewLogBackend(name, namespace, exportedFilePath string) *Backend {
	return &Backend{
		name:             name,
		namespace:        namespace,
		exportedFilePath: exportedFilePath,
	}
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) ConfigMap(name string) *BackendConfigMap {
	return NewBackendConfigMap(name, b.namespace, b.exportedFilePath, b.signalType)
}

func (b *Backend) LogBackendConfigMap(name string) *LogBackendConfigMap {
	return NewLogBackendConfigMap(name, b.namespace, b.exportedFilePath)
}

func (b *Backend) FluentDConfigMap(name string) *FluentDConfigMap {
	return NewFluentDConfigMap(name, b.namespace)
}

func (b *Backend) LogDeployment(configMapName, fluentDConfigMapName string) *BackendDeployment {
	return NewLogBackendDeployment(b.name, b.namespace, configMapName, filepath.Dir(b.exportedFilePath), fluentDConfigMapName)
}

func (b *Backend) Deployment(configMapName string) *BackendDeployment {
	return NewBackendDeployment(b.name, b.namespace, configMapName, filepath.Dir(b.exportedFilePath))
}

func (b *Backend) ExternalService() *ExternalBackendService {
	return NewExternalBackendService(b.name, b.namespace)
}

func (b *Backend) LogSpammer() *LogSpammer {
	return NewLogSpammer(b.name, b.namespace)
}
