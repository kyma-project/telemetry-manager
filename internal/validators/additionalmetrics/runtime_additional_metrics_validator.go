package additionalmetrics

import (
	"fmt"
	"slices"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricagent"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

var (
	validMetrics = append(metricagent.KubeletStatsReceiverMetrics, metricagent.K8sClusterReceiverMetrics...)
	docuLink     = "https://kyma-project.io/external-content/telemetry-manager/docs/user/collecting-metrics/runtime-metrics.html#runtime-metrics"
)

type Validator struct{}

func (v *Validator) Validate(mp telemetryv1beta1.MetricPipeline) error {
	if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) {
		return nil
	}

	additionalMetrics := mp.Spec.Input.Runtime.AdditionalMetrics

	if len(additionalMetrics) == 0 {
		return nil
	}

	for _, m := range additionalMetrics {
		if !slices.Contains(validMetrics, m) {
			return fmt.Errorf("invalid runtime additional metric: %s. For the list of valid metrics, check documentation: %s", m, docuLink)
		}
	}

	return nil
}
