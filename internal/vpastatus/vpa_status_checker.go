package vpastatus

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

type Checker struct {
	restConfig *rest.Config
}

func NewChecker(restConfig *rest.Config) *Checker {
	return &Checker{restConfig: restConfig}
}

// IsVpaActive checks if VPA is active based on the existence of the VPA CRD in the cluster
func (c *Checker) IsVpaActive(ctx context.Context, client client.Client, telemetryNamespace string) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c.restConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create discovery client: %w", err)
	}

	apiResourceList, err := discoveryClient.ServerResourcesForGroupVersion(names.VpaGroupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get server resources for group version %s: %w", names.VpaGroupVersion, err)
	}

	for _, r := range apiResourceList.APIResources {
		if r.Kind == names.VpaKind {
			return true, nil
		}
	}

	return false, nil
}
