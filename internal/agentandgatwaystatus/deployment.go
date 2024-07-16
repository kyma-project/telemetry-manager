package agentandgatwaystatus

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeploymentProber struct {
	client.Client
}

func (dp *DeploymentProber) IsReady(ctx context.Context, name types.NamespacedName) (bool, error) {
	var d appsv1.Deployment
	if err := dp.Get(ctx, name, &d); err != nil {
		return false, fmt.Errorf("failed to get %s/%s Deployment: %w", name.Namespace, name.Name, err)
	}

	desiredReplicas := *d.Spec.Replicas
	var allReplicaSets appsv1.ReplicaSetList

	listOps := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(d.Spec.Selector.MatchLabels),
		Namespace:     d.Namespace,
	}
	if err := dp.List(ctx, &allReplicaSets, listOps); err != nil {
		return false, fmt.Errorf("failed to list ReplicaSets: %w", err)
	}

	if err := dp.Get(ctx, name, &d); err != nil {
		return false, fmt.Errorf("failed to get %s/%s ReplicaSet for deployment: %w", name.Namespace, name.Name, err)
	}

	if d.Status.UpdatedReplicas == desiredReplicas {
		return true, nil
	}
	if err := checkPodStatus(ctx, dp.Client, name.Namespace, d.Spec.Selector); err != nil {
		return false, err
	}
	return true, nil
}
