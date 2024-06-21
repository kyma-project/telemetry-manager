package stubs

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

type GatewayResourcesHandler struct {
	ApplyFuncCalled  bool
	DeleteFuncCalled bool
}

func (grh *GatewayResourcesHandler) ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error {
	grh.ApplyFuncCalled = true
	return nil
}

func (grh *GatewayResourcesHandler) DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error {
	grh.DeleteFuncCalled = true
	return nil
}
