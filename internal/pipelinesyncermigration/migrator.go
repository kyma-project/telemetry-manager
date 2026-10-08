// Package pipelinesyncermigration deletes legacy pipeline syncer ConfigMaps.
//
// Before the syncer ConfigMap names were aligned with the lock naming convention,
// the names had a redundant "-sync-" infix (e.g. telemetry-logpipeline-sync-syncer).
// The new names drop the infix (e.g. telemetry-logpipeline-syncer). Because the old
// ConfigMaps carry owner references to live pipelines, Kubernetes GC will not remove
// them automatically. This migrator deletes them explicitly so the controller
// recreates them under the correct names on the next reconcile.
package pipelinesyncermigration

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const retryInterval = 10 * time.Second

var legacySyncerNames = []string{
	"telemetry-logpipeline-sync-syncer",
	"telemetry-metricpipeline-sync-syncer",
	"telemetry-tracepipeline-sync-syncer",
}

type Migrator struct {
	client    client.Client
	logger    logr.Logger
	namespace string
}

func New(c client.Client, logger logr.Logger, namespace string) *Migrator {
	return &Migrator{
		client:    c,
		logger:    logger.WithName("pipeline-syncer-migration"),
		namespace: namespace,
	}
}

func (m *Migrator) Start(ctx context.Context) error {
	for {
		err := m.deleteOldSyncersIfNeeded(ctx)
		if err == nil {
			return nil
		}

		m.logger.Error(err, "Legacy pipeline syncer cleanup failed, will retry", "retryInterval", retryInterval)

		select {
		case <-ctx.Done():
			m.logger.Info("Legacy pipeline syncer cleanup stopped due to context cancellation")
			return nil
		case <-time.After(retryInterval):
		}
	}
}

func (m *Migrator) deleteOldSyncersIfNeeded(ctx context.Context) error {
	for _, name := range legacySyncerNames {
		if err := m.deleteIfExists(ctx, name); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) deleteIfExists(ctx context.Context, name string) error {
	key := types.NamespacedName{Name: name, Namespace: m.namespace}

	var cm corev1.ConfigMap
	if err := m.client.Get(ctx, key, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get legacy syncer ConfigMap %q: %w", name, err)
	}

	m.logger.Info("Deleting legacy pipeline syncer ConfigMap", "name", name)

	if err := m.client.Delete(ctx, &cm); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete legacy syncer ConfigMap %q: %w", name, err)
	}

	return nil
}
