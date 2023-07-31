package verifiers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsDaemonSetReady(ctx context.Context, k8sClient client.Client, name types.NamespacedName) (bool, error) {
	var daemonSet appsv1.DaemonSet
	err := k8sClient.Get(ctx, name, &daemonSet)
	if err != nil {
		return false, err
	}
	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(daemonSet.Spec.Selector.MatchLabels),
		Namespace:     name.Namespace,
	}
	return IsPodReady(ctx, k8sClient, listOptions)
}
