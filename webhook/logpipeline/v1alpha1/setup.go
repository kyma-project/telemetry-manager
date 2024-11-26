package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewWebhookManagedBy(mgr).For(&telemetryv1alpha1.LogPipeline{}).
		WithDefaulter(&defaulter{
			ApplicationInputEnabled:          true,
			ApplicationInputKeepOriginalBody: true,
		}).
		Complete(); err != nil {
		return err
	}

	mgr.GetWebhookServer().Register("/validate-logpipeline", &webhook.Admission{
		Handler: newValidateHandler(mgr.GetClient(), mgr.GetScheme()),
	})

	return nil
}
