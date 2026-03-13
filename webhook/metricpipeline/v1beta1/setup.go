package v1beta1

import (
	ctrl "sigs.k8s.io/controller-runtime"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

func SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &telemetryv1beta1.MetricPipeline{}).
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
			OTLPInputEnabled:          true,
			DefaultOTLPOutputProtocol: telemetryv1beta1.OTLPProtocolGRPC,
			DiagnosticMetricsEnabled:  false,
			EnvoyMetricsEnabled:       false,
		}).
		WithValidator(&validator{}).
		Complete()
}
