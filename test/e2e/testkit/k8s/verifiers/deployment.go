//go:build e2e

package verifiers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsDeploymentReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var deployment appsv1.Deployment
	err := k8sClient.Get(ctx, name, &deployment)
	if err != nil {
		return false, err
	}
	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}
	var pods corev1.PodList
	err = k8sClient.List(ctx, &pods, &listOptions)
	if err != nil {
		return false, err
	}
	for _, pod := range pods.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil {
				return false, nil
			}
		}
	}
	return true, nil
}
