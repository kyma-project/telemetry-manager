package workloadstatus

import (
	"context"
	"errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrDeploymentNotFound = errors.New("deployment is not yet created")
	ErrDeploymentFetching = errors.New("failed to get Deployment")
)

type DeploymentProber struct {
	client.Client
}

func (dp *DeploymentProber) IsReady(ctx context.Context, name types.NamespacedName) error {
	log := logf.FromContext(ctx)
	var d appsv1.Deployment
	if err := dp.Get(ctx, name, &d); err != nil {
		if apierrors.IsNotFound(err) {
			// The status of pipeline is changed before the creation of daemonset
			log.V(1).Info("DaemonSet is not yet created")
			return ErrDeploymentNotFound
		}
		return ErrDeploymentFetching
	}

	desiredReplicas := *d.Spec.Replicas

	if d.Status.UpdatedReplicas == desiredReplicas {
		return nil
	}
	if err := checkPodStatus(ctx, dp.Client, name.Namespace, d.Spec.Selector); err != nil {
		return err
	}
	return nil
}
