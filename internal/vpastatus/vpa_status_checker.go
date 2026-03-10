package vpastatus

import (
	"context"
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Checker struct {
	restConfig *rest.Config
}

func NewChecker(restConfig *rest.Config) *Checker {
	return &Checker{restConfig: restConfig}
}

// IsVpaActive checks if VPA is active. VPA is considered active if the following 2 conditions are satisfied:
// 1. VPA CRD exists in the cluster
// 2. The annotation "telemetry.kyma-project.io/enable-vpa" is set to "true" on the Telemetry CR
func (c *Checker) IsVpaActive(ctx context.Context, client client.Client, telemetryNamespace string) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c.restConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create discovery client: %w", err)
	}

	apiResourceList, err := discoveryClient.ServerResourcesForGroupVersion(names.VpaGroupVersion)
	if err != nil {
		return false, nil
	}

	vpaCRDExists := false
	for _, r := range apiResourceList.APIResources {
		if r.Kind == names.VpaKind {
			vpaCRDExists = true
			break
		}
	}

	isVpaEnabled := telemetryutils.IsVpaEnabledInTelemetry(ctx, client, telemetryNamespace)

	return vpaCRDExists && isVpaEnabled, nil
}
