// Package logpipelinelockmigration cleans up the legacy shared LogPipeline lock.
//
// Before separate pipeline limits for OTel and FluentBit LogPipelines were
// introduced, both backends shared a single lock ConfigMap
// (telemetry-logpipeline-lock). After the split, FluentBit pipelines moved to a
// dedicated lock, but the old ConfigMap still carries stale owner references for
// FluentBit pipelines that nobody removes. Those phantom owners count against the
// OTel MaxPipelineCount, wrongly rejecting OTel pipelines with
// ErrMaxPipelinesExceeded.
//
// The migrator deletes the old lock when it still contains any FluentBit-mode
// owner reference. The LogPipeline controller recreates it cleanly on the next
// reconcile, with only OTel pipelines as owners. Once the lock holds no
// FluentBit owners, the migrator no-ops, so it is safe to retry.
package logpipelinelockmigration

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

const retryInterval = 10 * time.Second

type Migrator struct {
	client    client.Client
	logger    logr.Logger
	namespace string
}

func New(c client.Client, logger logr.Logger, namespace string) *Migrator {
	return &Migrator{
		client:    c,
		logger:    logger.WithName("logpipeline-lock-migration"),
		namespace: namespace,
	}
}

func (m *Migrator) Start(ctx context.Context) error {
	for {
		err := m.cleanupOldLockIfNeeded(ctx)
		if err == nil {
			return nil
		}

		m.logger.Error(err, "Legacy LogPipeline lock cleanup failed, will retry", "retryInterval", retryInterval)

		select {
		case <-ctx.Done():
			m.logger.Info("Legacy LogPipeline lock cleanup stopped due to context cancellation")
			return nil
		case <-time.After(retryInterval):
			// Continue with retry
		}
	}
}

func (m *Migrator) cleanupOldLockIfNeeded(ctx context.Context) error {
	lockName := types.NamespacedName{Name: names.LogPipelineLock, Namespace: m.namespace}

	var lock corev1.ConfigMap
	if err := m.client.Get(ctx, lockName, &lock); err != nil {
		if apierrors.IsNotFound(err) {
			// No legacy lock present, nothing to migrate.
			return nil
		}

		return fmt.Errorf("failed to get legacy LogPipeline lock: %w", err)
	}

	hasFluentBitOwner, err := m.lockHasFluentBitOwner(ctx, &lock)
	if err != nil {
		return err
	}

	if !hasFluentBitOwner {
		// The lock is already clean (only OTel owners, or none). Fixpoint reached.
		return nil
	}

	m.logger.Info("Legacy LogPipeline lock contains FluentBit owner references, deleting it so the controller rebuilds it cleanly", "lock", lockName)

	if err := m.client.Delete(ctx, &lock); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete legacy LogPipeline lock: %w", err)
	}

	return nil
}

// lockHasFluentBitOwner reports whether any owner reference on the lock resolves
// to a FluentBit-mode LogPipeline. Owner references whose LogPipeline cannot be
// resolved (e.g. already deleted) are treated as non-FluentBit to avoid deleting
// the lock on ambiguity.
func (m *Migrator) lockHasFluentBitOwner(ctx context.Context, lock *corev1.ConfigMap) (bool, error) {
	ownerRefs := lock.GetOwnerReferences()
	if len(ownerRefs) == 0 {
		return false, nil
	}

	var pipelineList telemetryv1beta1.LogPipelineList
	if err := m.client.List(ctx, &pipelineList); err != nil {
		return false, fmt.Errorf("failed to list LogPipelines: %w", err)
	}

	modeByUID := make(map[types.UID]logpipelineutils.Mode, len(pipelineList.Items))
	for i := range pipelineList.Items {
		lp := &pipelineList.Items[i]
		modeByUID[lp.UID] = logpipelineutils.PipelineMode(lp)
	}

	for _, ref := range ownerRefs {
		if mode, ok := modeByUID[ref.UID]; ok && mode == logpipelineutils.FluentBit {
			return true, nil
		}
	}

	return false, nil
}
