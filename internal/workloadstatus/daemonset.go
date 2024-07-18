package workloadstatus

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type DaemonSetProber struct {
	client.Client
}

type DaemonSetFetchingError struct {
	Name      string
	Namespace string
	Err       error
}

func (dsfe *DaemonSetFetchingError) Error() string {
	return fmt.Sprintf("failed to get %s/%s DaemonSet: %s", dsfe.Namespace, dsfe.Name, dsfe.Err)
}

func IsDaemonSetFetchingError(err error) bool {
	var dfse *DaemonSetFetchingError
	return errors.As(err, &dfse)
}

func (dsp *DaemonSetProber) IsReady(ctx context.Context, name types.NamespacedName) (bool, error) {
	log := logf.FromContext(ctx)

	var ds appsv1.DaemonSet
	if err := dsp.Get(ctx, name, &ds); err != nil {
		if apierrors.IsNotFound(err) {
			// The status of pipeline is changed before the creation of daemonset
			log.V(1).Info("DaemonSet is not yet created")
			return false, nil
		}
		return false, &DaemonSetFetchingError{
			Name:      name.Name,
			Namespace: name.Namespace,
			Err:       err,
		}
	}
	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
		return true, nil
	}

	if err := checkPodStatus(ctx, dsp.Client, name.Namespace, ds.Spec.Selector); err != nil {
		return false, err
	}

	// In a rolling update we perform update of one pod at a time. So, if the difference between desired and ready pods is 1, then we can consider the DaemonSet as ready.
	if ds.Status.DesiredNumberScheduled-ds.Status.NumberReady == 1 || ds.Status.DesiredNumberScheduled-ds.Status.NumberReady == 0 {
		return true, nil
	}

	return true, nil
}
