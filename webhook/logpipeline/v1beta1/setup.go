package v1beta1

import (
	ctrl "sigs.k8s.io/controller-runtime"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&telemetryv1beta1.LogPipeline{}).
		WithDefaulter(&defaulter{
			ExcludeNamespaces:            []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
			RuntimeInputEnabled:          true,
			RuntimeInputKeepOriginalBody: true,
			DefaultOTLPOutputProtocol:    telemetryv1beta1.OTLPProtocolGRPC,
			OTLPInputEnabled:             true,
		}).
		Complete()
}
