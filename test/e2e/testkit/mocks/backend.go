//go:build e2e

package mocks

import (
	"path/filepath"
)

type Backend struct {
	name         string
	namespace    string
	pipelineName string
	dataPath     string
}

func NewBackend(name, namespace, pipelineName, dataPath string) *Backend {
	return &Backend{
		name:         name,
		namespace:    namespace,
		pipelineName: pipelineName,
		dataPath:     dataPath,
	}
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) ConfigMap(name string) *BackendConfigMap {
	return NewBackendConfigMap(name, b.namespace, b.pipelineName, b.dataPath)
}

func (b *Backend) Deployment(configMapName string) *BackendDeployment {
	return NewBackendDeployment(b.name, b.namespace, configMapName, filepath.Dir(b.dataPath))
}

func (b *Backend) ExternalService() *ExternalBackendService {
	return NewExternalBackendService(b.name, b.namespace)
}
