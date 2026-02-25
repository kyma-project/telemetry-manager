package otlpgateway

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

// OTLPGatewayConfigBuilder builds OTel Collector configuration for the OTLP Gateway.
type OTLPGatewayConfigBuilder interface {
	Build(ctx context.Context, opts otlpgateway.BuildOptions) (*common.Config, common.EnvVars, error)
}

// GatewayApplierDeleter manages the lifecycle of OTLP Gateway resources (DaemonSet, ConfigMap, Secret, etc.).
type GatewayApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

// Prober checks the health and readiness of gateway workloads.
type Prober interface {
	IsReady(ctx context.Context, name types.NamespacedName) error
}

// IstioStatusChecker checks whether Istio is active in the cluster.
type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

// ErrorToMessageConverter converts errors to human-readable status messages.
type ErrorToMessageConverter interface {
	Convert(err error) string
}
