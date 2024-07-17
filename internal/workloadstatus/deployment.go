package workloadstatus

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type DeploymentProber struct {
	client.Client
}

func (dp *DeploymentProber) IsReady(ctx context.Context, name types.NamespacedName) (bool, error) {
	log := logf.FromContext(ctx)
	var d appsv1.Deployment
	if err := dp.Get(ctx, name, &d); err != nil {
		if apierrors.IsNotFound(err) {
			// The status of pipeline is changed before the creation of daemonset
			log.V(1).Info("DaemonSet is not yet created")
			return false, nil
		}
		return false, fmt.Errorf("failed to get %s/%s Deployment: %w", name.Namespace, name.Name, err)
	}

	desiredReplicas := *d.Spec.Replicas

	if d.Status.UpdatedReplicas == desiredReplicas {
		return true, nil
	}
	if err := checkPodStatus(ctx, dp.Client, name.Namespace, d.Spec.Selector); err != nil {
		return false, err
	}
	return true, nil
}
