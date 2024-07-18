package workloadstatus

import (
	"context"
	"errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrDaemonSetNotFound      = errors.New("DaemonSet is not yet created")
	ErrDaemonSetFetchingError = errors.New("failed to get DaemonSet")
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
		return ErrDaemonSetFetchingError
	}
	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
		return nil
	}

	if err := checkPodStatus(ctx, dsp.Client, name.Namespace, ds.Spec.Selector); err != nil {
		return err
	}

	// In a rolling update we perform update of one pod at a time. So, if the difference between desired and ready pods is 1, then we can consider the DaemonSet as ready.
	if ds.Status.DesiredNumberScheduled-ds.Status.NumberReady == 1 || ds.Status.DesiredNumberScheduled-ds.Status.NumberReady == 0 {
		return nil
	}

	return nil
}
