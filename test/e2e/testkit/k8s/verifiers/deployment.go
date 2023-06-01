//go:build e2e

package verifiers

import (
	"context"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsDeploymentReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	pods, err := k8s.ListDeploymentPods(ctx, k8sClient, name)
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil {
				return false, nil
			}
		}
	}
	return true, nil
}
