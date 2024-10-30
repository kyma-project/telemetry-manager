package metricpipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	utils "github.com/kyma-project/telemetry-manager/utils/shared"
)

func GetSecretRefs(mp *telemetryv1alpha1.MetricPipeline) []telemetryv1alpha1.SecretKeyRef {
	return utils.GetRefsInOTLPOutput(mp.Spec.Output.OTLP)
}
