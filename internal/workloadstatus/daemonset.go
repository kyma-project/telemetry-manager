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

type DaemonSetProber struct {
	client.Client
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
		return false, fmt.Errorf("failed to get %s/%s DaemonSet: %w", name.Namespace, name.Name, err)
	}
	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
		return true, nil
	}

	if err := checkPodStatus(ctx, dsp.Client, name.Namespace, ds.Spec.Selector); err != nil {
		return false, err
	}

	return true, nil
}
