package telemetry

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const defaultCollectionInterval = 30 * time.Second

// MetricCollectionIntervals holds the resolved collection interval for each pull-based metric input type.
type MetricCollectionIntervals struct {
	Runtime    time.Duration
	Prometheus time.Duration
	Istio      time.Duration
}

// ResolveMetricCollectionIntervals computes the effective collection interval for each input type
// from the Telemetry CR MetricSpec, following the precedence:
// input-specific override > metric.collectionInterval > 30s default.
func ResolveMetricCollectionIntervals(metricSpec *operatorv1beta1.MetricSpec) MetricCollectionIntervals {
	globalInterval := defaultCollectionInterval

	if metricSpec != nil && metricSpec.CollectionInterval != nil {
		globalInterval = metricSpec.CollectionInterval.Duration
	}

	intervals := MetricCollectionIntervals{
		Runtime:    globalInterval,
		Prometheus: globalInterval,
		Istio:      globalInterval,
	}

	if metricSpec != nil {
		if metricSpec.Runtime != nil && metricSpec.Runtime.CollectionInterval != nil {
			intervals.Runtime = metricSpec.Runtime.CollectionInterval.Duration
		}

		if metricSpec.Prometheus != nil && metricSpec.Prometheus.CollectionInterval != nil {
			intervals.Prometheus = metricSpec.Prometheus.CollectionInterval.Duration
		}

		if metricSpec.Istio != nil && metricSpec.Istio.CollectionInterval != nil {
			intervals.Istio = metricSpec.Istio.CollectionInterval.Duration
		}
	}

	return intervals
}

func GetDefaultTelemetryInstance(ctx context.Context, client client.Client, namespace string) (operatorv1beta1.Telemetry, error) {
	var telemetry operatorv1beta1.Telemetry

	telemetryName := types.NamespacedName{
		Namespace: namespace,
		Name:      names.DefaultTelemetry,
	}

	if err := client.Get(ctx, telemetryName, &telemetry); err != nil {
		return telemetry, err
	}

	return telemetry, nil
}

type Options struct {
	SignalType                common.SignalType
	Client                    client.Client
	DefaultReplicas           int32
	DefaultTelemetryNamespace string
}

// GetReplicaCountFromTelemetry retrieves the desired number of gateway replicas from the Telemetry CR
// for the specified signal type (traces, logs, or metrics).
// It returns the configured replica count if static scaling is configured, otherwise returns the default replica count.
func GetReplicaCountFromTelemetry(ctx context.Context, opts Options) int32 {
	telemetry, err := GetDefaultTelemetryInstance(ctx, opts.Client, opts.DefaultTelemetryNamespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default scaling")
		return opts.DefaultReplicas
	}

	gatewaySpec := getGatewaySpec(telemetry.Spec, opts.SignalType)
	if gatewaySpec != nil &&
		gatewaySpec.Scaling.Type == operatorv1beta1.StaticScalingStrategyType &&
		gatewaySpec.Scaling.Static != nil &&
		gatewaySpec.Scaling.Static.Replicas > 0 {
		return gatewaySpec.Scaling.Static.Replicas
	}

	return opts.DefaultReplicas
}

// getGatewaySpec returns the GatewaySpec for the given signal type, or nil if not configured.
func getGatewaySpec(spec operatorv1beta1.TelemetrySpec, signalType common.SignalType) *operatorv1beta1.GatewaySpec {
	switch signalType {
	case common.SignalTypeTrace:
		if spec.Trace != nil {
			return &spec.Trace.Gateway
		}
	case common.SignalTypeLog:
		if spec.Log != nil {
			return &spec.Log.Gateway
		}
	case common.SignalTypeMetric:
		if spec.Metric != nil {
			return &spec.Metric.Gateway
		}
	}

	return nil
}

// GetClusterNameFromTelemetry retrieves the cluster name from the Telemetry CR enrichment configuration.
// If no custom cluster name is configured, it returns the provided default name.
func GetClusterNameFromTelemetry(ctx context.Context, opts Options) string {
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, opts.Client)
	defaultClusterName := shootInfo.ClusterName

	telemetry, err := GetDefaultTelemetryInstance(ctx, opts.Client, opts.DefaultTelemetryNamespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default shoot name as cluster name")
		return defaultClusterName
	}

	if telemetry.Spec.Enrichments != nil &&
		telemetry.Spec.Enrichments.Cluster != nil &&
		telemetry.Spec.Enrichments.Cluster.Name != "" {
		return telemetry.Spec.Enrichments.Cluster.Name
	}

	return defaultClusterName
}

// GetServiceEnrichmentFromTelemetryOrDefault retrieves the service enrichment strategy from the Telemetry CR service-enrichment annotation.
// If no valid annotation is found, it returns the provided default service enrichment strategy.
func GetServiceEnrichmentFromTelemetryOrDefault(ctx context.Context, opts Options) string {
	telemetry, err := GetDefaultTelemetryInstance(ctx, opts.Client, opts.DefaultTelemetryNamespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: default service enrichment strategy will be used")
		return commonresources.AnnotationValueTelemetryServiceEnrichmentDefault
	}

	if telemetry.Annotations != nil {
		if value, ok := telemetry.Annotations[commonresources.AnnotationKeyTelemetryServiceEnrichment]; ok {
			if value == commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy ||
				value == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return value
			}
		}
	}

	return commonresources.AnnotationValueTelemetryServiceEnrichmentDefault
}

// IsVpaEnabledInTelemetry checks if VPA is enabled via the "telemetry.kyma-project.io/enable-vpa" annotation on the Telemetry CR.
// Returns true by default, and false only if the annotation is explicitly set to "false".
func IsVpaEnabledInTelemetry(ctx context.Context, client client.Client, telemetryNamespace string) bool {
	telemetry, err := GetDefaultTelemetryInstance(ctx, client, telemetryNamespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: VPA will be enabled by default")
		return true
	}

	if telemetry.Annotations != nil {
		if value, ok := telemetry.Annotations[commonresources.AnnotationKeyEnableVPA]; ok {
			return value != commonresources.AnnotationValueFalse
		}
	}

	return true
}
