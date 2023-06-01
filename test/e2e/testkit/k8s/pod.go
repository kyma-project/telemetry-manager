package k8s

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListDeploymentPods(ctx context.Context, k8sClient client.Client, deploymentName types.NamespacedName) ([]*corev1.Pod, error) {
	var deployment appsv1.Deployment
	err := k8sClient.Get(ctx, deploymentName, &deployment)
	if err != nil {
		return nil, err
	}

	var pods corev1.PodList
	if err = k8sClient.List(ctx, &pods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
		Namespace:     deploymentName.Namespace,
	}); err != nil {
		return nil, err
	}

	var results []*corev1.Pod
	for _, pod := range pods.Items {
		results = append(results, &pod)
	}

	return results, nil
}
