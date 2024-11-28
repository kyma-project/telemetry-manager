package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func SetupWithManager(mgr ctrl.Manager) {
	mgr.GetWebhookServer().Register("/validate-logparser", &webhook.Admission{
		Handler: newValidateHandler(mgr.GetScheme()),
	})
}
