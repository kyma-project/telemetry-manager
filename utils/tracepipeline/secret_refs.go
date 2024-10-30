package tracepipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	utils "github.com/kyma-project/telemetry-manager/utils/shared"
)

func GetSecretRefs(tp *telemetryv1alpha1.TracePipeline) []telemetryv1alpha1.SecretKeyRef {
	return utils.GetRefsInOTLPOutput(tp.Spec.Output.OTLP)
}
