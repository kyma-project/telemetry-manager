package verifiers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsPodReady(ctx context.Context, k8sClient client.Client, listOptions client.ListOptions) (bool, error) {
	var pods corev1.PodList
	err := k8sClient.List(ctx, &pods, &listOptions)
	if err != nil {
		return false, fmt.Errorf("failed to list pods: %w", err)
	}
	for _, pod := range pods.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil {
				return false, fmt.Errorf("pod %s has a container %s that is not running", pod.Name, containerStatus.Name)
			}
		}
	}
	return true, nil
}

func HasContainer(ctx context.Context, k8sClient client.Client, listOptions client.ListOptions, containerName string) (bool, error) {
	var pods corev1.PodList
	err := k8sClient.List(ctx, &pods, &listOptions)
	if err != nil {
		return false, fmt.Errorf("failed to list pods: %w", err)
	}
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				return true, nil
			}
		}
	}
	return false, nil
}
