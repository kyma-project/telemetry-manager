package workloadstatus

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DaemonSetProber struct {
	client.Client
}

func (dsp *DaemonSetProber) IsReady(ctx context.Context, name types.NamespacedName) error {
	var ds appsv1.DaemonSet
	if err := dsp.Get(ctx, name, &ds); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrDaemonSetNotFound
		}

		return ErrDaemonSetFetching
	}

	updated := ds.Status.UpdatedNumberScheduled
	desired := ds.Status.DesiredNumberScheduled
	ready := ds.Status.NumberReady

	if updated == desired && ready >= desired {
		return nil
	}

	// check if any of the pods have issues. If so return the error
	if err := checkPodStatus(ctx, dsp.Client, name.Namespace, ds.Spec.Selector); err != nil {
		return err
	}

	// Assume that there is an update or rollout of pods is in progress
	return &RolloutInProgressError{}
}
