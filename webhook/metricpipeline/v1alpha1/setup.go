package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&telemetryv1alpha1.MetricPipeline{}).
		WithDefaulter(&defaulter{
			ExcludeNamespaces: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
			RuntimeInputResources: runtimeInputResourceDefaults{
				Pod:         true,
				Container:   true,
				Node:        true,
				Volume:      true,
				DaemonSet:   true,
				Deployment:  true,
				StatefulSet: true,
				Job:         true,
			},
			DefaultOTLPOutputProtocol: telemetryv1alpha1.OTLPProtocolGRPC,
		}).
		Complete()
}
