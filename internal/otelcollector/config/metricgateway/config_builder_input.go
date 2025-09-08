package metricgateway

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDOTLPReceiver),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.OTLPReceiver{
				Protocols: common.ReceiverProtocols{
					HTTP: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
					},
					GRPC: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
					},
				},
			}
		},
	)
}

func (b *Builder) addKymaStatsReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDKymaStatsReceiver),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &KymaStatsReceiver{
				AuthType:           "serviceAccount",
				K8sLeaderElector:   "k8s_leader_elector",
				CollectionInterval: "30s",
				Resources: []ModuleGVR{
					{Group: "operator.kyma-project.io", Version: "v1alpha1", Resource: "telemetries"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "logpipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "tracepipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "metricpipelines"},
				},
			}
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1alpha1.MetricPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addInputRoutingExporter() buildComponentFunc {
	return b.AddExporter(
		formatRoutingConnectorID,
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return common.SkipEnrichmentRoutingConnectorConfig(
				[]string{formatEnrichmentServicePipelineID(mp)},
				[]string{formatOutputServicePipelineID(mp)},
			), nil, nil
		},
	)
}
