// TODO: Remove after rollout 1.60.0
package labelupdater

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

const retryInterval = 10 * time.Second

// Updater patches the module label onto resources that were created by an older
// version of the manager before the label-scoped informer cache was introduced.
// Without the label, the scoped cache cannot see these resources, causing
// CreateOrUpdate to fail with AlreadyExists on every reconciliation.
//
// It uses an API reader (bypassing the cache) to find the resources and the
// regular client to patch them so the label is persisted and the cache picks
// them up.
type Updater struct {
	apiReader client.Reader
	client    client.Client
	logger    logr.Logger
	namespace string
}

func New(apiReader client.Reader, c client.Client, logger logr.Logger, namespace string) *Updater {
	return &Updater{
		apiReader: apiReader,
		client:    c,
		logger:    logger.WithName("label-updater"),
		namespace: namespace,
	}
}

func (u *Updater) Start(ctx context.Context) error {
	for {
		err := u.ensureLabels(ctx)
		if err == nil {
			return nil
		}

		u.logger.Error(err, "Label update failed, will retry", "retryInterval", retryInterval)

		select {
		case <-ctx.Done():
			u.logger.Info("Label update stopped due to context cancellation")
			return nil
		case <-time.After(retryInterval):
			// Continue with retry
		}
	}
}

func (u *Updater) ensureLabels(ctx context.Context) error {
	u.logger.Info("Checking for resources missing the module label")

	resources := []client.Object{
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: names.FluentBit, Namespace: u.namespace}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: names.FluentBit}},
	}

	for _, r := range resources {
		if err := u.ensureLabelOnResource(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

func (u *Updater) ensureLabelOnResource(ctx context.Context, obj client.Object) error {
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Use the API reader to bypass the label-scoped cache
	if err := u.apiReader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get %T %s: %w", obj, key, err)
	}

	if hasModuleLabel(obj) {
		return nil
	}

	if err := patchModuleLabel(ctx, u.client, obj); err != nil {
		return fmt.Errorf("failed to patch label on %T %s: %w", obj, key, err)
	}

	u.logger.Info("Patched module label", "resource", key, "kind", fmt.Sprintf("%T", obj))

	return nil
}

func hasModuleLabel(obj client.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	_, ok := labels[commonresources.LabelKeyKymaModule]

	return ok
}

func patchModuleLabel(ctx context.Context, c client.Client, obj client.Object) error {
	base, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("failed to deep copy %T", obj)
	}

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[commonresources.LabelKeyKymaModule] = commonresources.LabelValueKymaModule
	obj.SetLabels(labels)

	return c.Patch(ctx, obj, client.MergeFrom(base))
}
