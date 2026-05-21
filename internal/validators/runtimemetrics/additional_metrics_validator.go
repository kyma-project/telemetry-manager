package runtimemetrics

import (
	"errors"
	"fmt"
	"slices"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricagent"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

var (
	validRuntimeMetrics = append(metricagent.KubeletStatsReceiverMetrics, metricagent.K8sClusterReceiverMetrics...)
	docuLink            = "https://kyma-project.io/external-content/telemetry-manager/docs/user/collecting-metrics/runtime-metrics.html#runtime-additional-metrics"
)

type InvalidAdditionalMetricError struct {
	Err error
}

func (e *InvalidAdditionalMetricError) Error() string {
	return e.Err.Error()
}

func IsInvalidAdditionalMetricError(err error) bool {
	var errInvalidAdditionalMetric *InvalidAdditionalMetricError
	return errors.As(err, &errInvalidAdditionalMetric)
}

type Validator struct{}

func (v *Validator) Validate(mp *telemetryv1beta1.MetricPipeline) error {
	if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) {
		return nil
	}

	additionalMetrics := mp.Spec.Input.Runtime.AdditionalMetrics

	if len(additionalMetrics) == 0 {
		return nil
	}

	for _, m := range additionalMetrics {
		if !slices.Contains(validRuntimeMetrics, m) {
			return &InvalidAdditionalMetricError{
				Err: fmt.Errorf("invalid runtime additional metric: %s. For the list of valid runtime additional metrics, check documentation: %s", m, docuLink),
			}
		}
	}

	return nil
}
