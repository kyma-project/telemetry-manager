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
	ErrReplicaCountMismatch   = errors.New("replica count mismatch")
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

	updated := ds.Status.UpdatedNumberScheduled
	desired := ds.Status.DesiredNumberScheduled
	ready := ds.Status.NumberReady
	if updated == desired && ready >= desired {
		return nil
	}

	if err := checkPodStatus(ctx, dsp.Client, name.Namespace, ds.Spec.Selector); err != nil {
		return err
	}

	return ErrReplicaCountMismatch
}
