package stubs

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

type AgentResourcesHandler struct {
	ApplyFuncCalled  bool
	DeleteFuncCalled bool
}

func (arh *AgentResourcesHandler) ApplyResources(ctx context.Context, c client.Client, opts otelcollector.AgentApplyOptions) error {
	arh.ApplyFuncCalled = true
	return nil
}

func (arh *AgentResourcesHandler) DeleteResources(ctx context.Context, c client.Client) error {
	arh.DeleteFuncCalled = true
	return nil
}
