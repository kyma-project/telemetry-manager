package otlpgateway

import (
	"context"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

// OTLPGatewayConfigBuilder builds OTel Collector configuration for the OTLP Gateway.
type OTLPGatewayConfigBuilder interface {
	Build(ctx context.Context, opts otlpgateway.BuildOptions) (*common.Config, common.EnvVars, error)
}

// GatewayApplierDeleter manages the lifecycle of OTLP Gateway resources (DaemonSet, ConfigMap, Secret, etc.).
type GatewayApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool, vpaCRDExists bool) error
}

// IstioStatusChecker checks whether Istio is active in the cluster.
type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) (bool, error)
}

// VpaStatusChecker determines whether Vertical Pod Autoscaler (VPA) is active in the cluster.
type VpaStatusChecker interface {
	// VpaCRDExists checks if the VPA CRD exists in the cluster.
	VpaCRDExists(ctx context.Context, client client.Client) (bool, error)
}

// NodeSizeTracker tracks node sizes and provides VPA memory calculations.
type NodeSizeTracker interface {
	// VPAMaxAllowedMemory returns 15% of the smallest allocatable memory, rounded down to the nearest KiB.
	VPAMaxAllowedMemory() resource.Quantity
}

// OverridesHandler loads the override configuration for the OTLP Gateway.
type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}
