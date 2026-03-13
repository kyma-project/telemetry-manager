package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

func SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &telemetryv1alpha1.MetricPipeline{}).
		WithDefaulter(&defaulter{
			ExcludeNamespaces: namespaces.System(),
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
			DiagnosticMetricsEnabled:  false,
			EnvoyMetricsEnabled:       false,
		}).
		WithValidator(&validator{}).
		Complete()
}
