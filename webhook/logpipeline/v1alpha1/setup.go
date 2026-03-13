package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &telemetryv1alpha1.LogPipeline{}).
		WithDefaulter(&defaulter{
			ApplicationInputEnabled:          true,
			ApplicationInputKeepOriginalBody: true,
			DefaultOTLPProtocol:              telemetryv1alpha1.OTLPProtocolGRPC,
		}).
		WithValidator(&validator{}).
		Complete()
}
