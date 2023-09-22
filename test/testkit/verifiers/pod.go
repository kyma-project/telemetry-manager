package verifiers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsPodReady(ctx context.Context, k8sClient client.Client, listOptions client.ListOptions) (bool, error) {
	var pods corev1.PodList
	err := k8sClient.List(ctx, &pods, &listOptions)
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

func HasContainer(ctx context.Context, k8sClient client.Client, listOptions client.ListOptions, containerName string) (bool, error) {
	var pods corev1.PodList
	err := k8sClient.List(ctx, &pods, &listOptions)
	if err != nil {
		return false, err
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
