package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DaemonSetAnnotator struct {
	client.Client
}

func (dsa *DaemonSetAnnotator) SetAnnotation(ctx context.Context, name types.NamespacedName, key, value string) error {
	var ds appsv1.DaemonSet
	if err := dsa.Get(ctx, name, &ds); err != nil {
		return fmt.Errorf("failed to get %s/%s DaemonSet: %w", name.Namespace, name.Name, err)
	}

	patchedDS := *ds.DeepCopy()
	if patchedDS.Spec.Template.ObjectMeta.Annotations == nil {
		patchedDS.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	} else if patchedDS.Spec.Template.ObjectMeta.Annotations[key] == value {
		return nil
	}

	patchedDS.Spec.Template.ObjectMeta.Annotations[key] = value

	if err := dsa.Patch(ctx, &patchedDS, client.MergeFrom(&ds)); err != nil {
		return fmt.Errorf("failed to patch %s/%s DaemonSet: %w", name.Namespace, name.Name, err)
	}

	return nil
}
