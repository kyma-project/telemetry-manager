package kubernetes

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
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
		return false, fmt.Errorf("failed to get %s/%s DaemonSet: %v", name.Namespace, name.Name, err)
	}

	generation := ds.Generation
	observedGeneration := ds.Status.ObservedGeneration
	updated := ds.Status.UpdatedNumberScheduled
	desired := ds.Status.DesiredNumberScheduled
	ready := ds.Status.NumberReady

	log.V(1).Info(fmt.Sprintf("Checking DaemonSet: updated: %d, desired: %d, ready: %d, generation: %d, observed generation: %d",
		updated, desired, ready, generation, observedGeneration), "name", name.Name)

	return observedGeneration == generation && updated == desired && ready >= desired, nil
}

func (dsp *DaemonSetProber) Status(ctx context.Context, name types.NamespacedName) (string, error) {
	log := logf.FromContext(ctx)

	var ds appsv1.DaemonSet
	if err := dsp.Get(ctx, name, &ds); err != nil {
		if apierrors.IsNotFound(err) {
			// The status of pipeline is changed before the creation of daemonset
			log.V(1).Info("DaemonSet is not yet created")
			return reconciler.ReasonFluentBitDSNotReady, nil
		}
		return "", fmt.Errorf("failed to get %s/%s DaemonSet: %v", name.Namespace, name.Name, err)
	}

	listOps := &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(ds.Spec.Selector.MatchLabels),
		Namespace:     ds.Namespace,
	}
	var podsList *corev1.PodList
	if err := dsp.List(ctx, podsList, listOps); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Pods for daemon set: %v are not yet created", ds.Name)
			return reconciler.ReasonFluentBitDSNotReady, nil
		}
		return "", fmt.Errorf("unable to find pods for daemonset: %v: %w", ds.Name, err)
	}
	if len(podsList.Items) != 0 {
		return podStatus(podsList.Items)
	}

	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
		return reconciler.ReasonFluentBitDSReady, nil
	}

	return reconciler.ReasonFluentBitDSNotReady, nil
}

type DaemonSetAnnotator struct {
	client.Client
}

func (dsa *DaemonSetAnnotator) SetAnnotation(ctx context.Context, name types.NamespacedName, key, value string) error {
	var ds appsv1.DaemonSet
	if err := dsa.Get(ctx, name, &ds); err != nil {
		return fmt.Errorf("failed to get %s/%s DaemonSet: %v", name.Namespace, name.Name, err)
	}

	patchedDS := *ds.DeepCopy()
	if patchedDS.Spec.Template.ObjectMeta.Annotations == nil {
		patchedDS.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	} else if patchedDS.Spec.Template.ObjectMeta.Annotations[key] == value {
		return nil
	}

	patchedDS.Spec.Template.ObjectMeta.Annotations[key] = value

	if err := dsa.Patch(ctx, &patchedDS, client.MergeFrom(&ds)); err != nil {
		return fmt.Errorf("failed to patch %s/%s DaemonSet: %v", name.Namespace, name.Name, err)
	}

	return nil
}
