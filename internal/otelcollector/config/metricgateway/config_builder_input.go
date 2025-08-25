package metricgateway

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func (b *Builder) addInputServicePipeline(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatInputMetricServicePipelineID(mp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, mp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addInputReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(b.config, componentIDFunc, configFunc, formatInputMetricServicePipelineID)
}

func (b *Builder) addInputProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(b.config, componentIDFunc, configFunc, formatInputMetricServicePipelineID)
}

func (b *Builder) addInputExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(b.config, b.envVars, componentIDFunc, configFunc, formatInputMetricServicePipelineID)
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.addInputReceiver(
		staticComponentID(common.ComponentIDOTLPReceiver),
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
	return b.addInputReceiver(
		staticComponentID(common.ComponentIDKymaStatsReceiver),
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

func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.addInputProcessor(
		staticComponentID(common.ComponentIDMemoryLimiterProcessor),
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
	return b.addInputExporter(
		formatRoutingConnectorID,
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return enrichmentRoutingConnectorConfig(mp), nil, nil
		},
	)
}
