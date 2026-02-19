package storagemigration

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

const (
	oldVersion    = "v1alpha1"
	targetVersion = "v1beta1"

	migrationTimeout = 5 * time.Minute

	retryBackoffInitial = 100 * time.Millisecond
	retryBackoffMax     = 5 * time.Second
	retryBackoffFactor  = 2.0
	retryBackoffSteps   = 10
)

var pipelineCRDs = []string{
	"logpipelines.telemetry.kyma-project.io",
	"metricpipelines.telemetry.kyma-project.io",
	"tracepipelines.telemetry.kyma-project.io",
}

// CRD name for the Telemetry operator CR
const telemetryCRD = "telemetries.operator.kyma-project.io"

type Migrator struct {
	client client.Client
	logger logr.Logger
}

func New(c client.Client, logger logr.Logger) *Migrator {
	return &Migrator{
		client: c,
		logger: logger.WithName("storage-migration"),
	}
}

func (m *Migrator) MigrateIfNeeded(ctx context.Context) error {
	if err := m.migratePipelinesIfNeeded(ctx); err != nil {
		m.logger.Error(err, "migration failed")
	}
	return nil
}

func (m *Migrator) migratePipelinesIfNeeded(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()

	m.logger.Info("Checking if storage version migration is needed")

	// Collect all CRDs that need migration
	allCRDs := append(pipelineCRDs, telemetryCRD) //nolint:gocritic // intentional append to new slice
	needsMigration := false

	for _, crdName := range allCRDs {
		needed, err := m.needsMigration(ctx, crdName)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", crdName, err)
		}

		if needed {
			needsMigration = true

			m.logger.Info("Migration needed for CRD", "crd", crdName)
		}
	}

	if !needsMigration {
		m.logger.Info("No storage version migration needed, all CRDs already clean")

		return nil
	}

	m.logger.Info("Starting storage version migration")

	if err := m.migrateLogPipelines(ctx); err != nil {
		return fmt.Errorf("failed to migrate LogPipelines: %w", err)
	}

	if err := m.migrateMetricPipelines(ctx); err != nil {
		return fmt.Errorf("failed to migrate MetricPipelines: %w", err)
	}

	if err := m.migrateTracePipelines(ctx); err != nil {
		return fmt.Errorf("failed to migrate TracePipelines: %w", err)
	}

	if err := m.migrateTelemetries(ctx); err != nil {
		return fmt.Errorf("failed to migrate Telemetries: %w", err)
	}

	// Clear old versions from all CRDs
	for _, crdName := range allCRDs {
		if err := m.clearStoredVersion(ctx, crdName); err != nil {
			return fmt.Errorf("failed to clear stored version for %s: %w", crdName, err)
		}
	}

	m.logger.Info("Storage version migration completed successfully")

	return nil
}

func (m *Migrator) needsMigration(ctx context.Context, crdName string) (bool, error) {
	var crd apiextensionsv1.CustomResourceDefinition

	err := m.retryOnStorageInit(ctx, func() error {
		return m.client.Get(ctx, types.NamespacedName{Name: crdName}, &crd)
	})
	if err != nil {
		return false, fmt.Errorf("failed to get CRD %s: %w", crdName, err)
	}

	return slices.Contains(crd.Status.StoredVersions, oldVersion), nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateLogPipelines(ctx context.Context) error {

	var list telemetryv1beta1.LogPipelineList
	err := m.client.List(ctx, &list)
	if err != nil {
		return fmt.Errorf("failed to list LogPipelines: %w", err)
	}

	m.logger.Info("Migrating LogPipelines", "count", len(list.Items))

	for i := range list.Items {
		if err := m.updateResourceWithRetry(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate LogPipeline %s: %w", list.Items[i].Name, err)
		}
	}

	return nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateMetricPipelines(ctx context.Context) error {
	var list telemetryv1beta1.MetricPipelineList

	err := m.retryOnStorageInit(ctx, func() error {
		return m.client.List(ctx, &list)
	})
	if err != nil {
		return fmt.Errorf("failed to list MetricPipelines: %w", err)
	}

	m.logger.Info("Migrating MetricPipelines", "count", len(list.Items))

	for i := range list.Items {
		if err := m.updateResourceWithRetry(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate MetricPipeline %s: %w", list.Items[i].Name, err)
		}
	}

	return nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateTracePipelines(ctx context.Context) error {
	var list telemetryv1beta1.TracePipelineList

	err := m.retryOnStorageInit(ctx, func() error {
		return m.client.List(ctx, &list)
	})
	if err != nil {
		return fmt.Errorf("failed to list TracePipelines: %w", err)
	}

	m.logger.Info("Migrating TracePipelines", "count", len(list.Items))

	for i := range list.Items {
		if err := m.updateResourceWithRetry(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate TracePipeline %s: %w", list.Items[i].Name, err)
		}
	}

	return nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateTelemetries(ctx context.Context) error {
	var list operatorv1beta1.TelemetryList

	err := m.retryOnStorageInit(ctx, func() error {
		return m.client.List(ctx, &list)
	})
	if err != nil {
		return fmt.Errorf("failed to list Telemetries: %w", err)
	}

	m.logger.Info("Migrating Telemetries", "count", len(list.Items))

	for i := range list.Items {
		if err := m.updateResourceWithRetry(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate Telemetry %s/%s: %w", list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}

	return nil
}

func (m *Migrator) updateResourceWithRetry(ctx context.Context, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	if err := m.client.Get(ctx, key, obj); err != nil {
		return fmt.Errorf("failed to get resource: %w", err)
	}

	if err := m.client.Update(ctx, obj); err != nil {
		if client.IgnoreNotFound(err) != nil {
			m.logger.V(1).Info("Retrying update due to conflict", "resource", key)

			return nil //nolint:nilerr // retry on conflict is intentional
		}

		return err
	}

	m.logger.V(1).Info("Migrated resource", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", key.Name)
	return nil

}

func (m *Migrator) clearStoredVersion(ctx context.Context, crdName string) error {
	backoff := wait.Backoff{
		Duration: retryBackoffInitial,
		Factor:   retryBackoffFactor,
		Cap:      retryBackoffMax,
		Steps:    retryBackoffSteps,
	}

	var lastErr error

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		var crd apiextensionsv1.CustomResourceDefinition
		if err := m.client.Get(ctx, types.NamespacedName{Name: crdName}, &crd); err != nil {
			// Retry on transient errors
			if apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) {
				m.logger.V(1).Info("Retrying Get due to transient error", "crd", crdName, "error", err)
				lastErr = err

				return false, nil
			}

			return false, fmt.Errorf("failed to get CRD %s: %w", crdName, err)
		}

		newStoredVersions := make([]string, 0, len(crd.Status.StoredVersions))

		for _, version := range crd.Status.StoredVersions {
			if version != oldVersion {
				newStoredVersions = append(newStoredVersions, version)
			}
		}

		if len(newStoredVersions) == len(crd.Status.StoredVersions) {
			m.logger.Info("StoredVersions already clean", "crd", crdName)

			return true, nil
		}

		crd.Status.StoredVersions = newStoredVersions

		if err := m.client.Status().Update(ctx, &crd); err != nil {
			lastErr = err
			// Retry on conflicts and transient errors
			if apierrors.IsConflict(err) || apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) {
				m.logger.V(1).Info("Retrying status update", "crd", crdName, "error", err)

				return false, nil
			}

			return false, fmt.Errorf("failed to update CRD status: %w", err)
		}

		m.logger.Info("Cleared old stored version", "crd", crdName, "removedVersion", oldVersion)

		return true, nil
	})

	if wait.Interrupted(err) {
		return fmt.Errorf("timed out clearing stored version for %s: %w", crdName, lastErr)
	}

	return err
}

// retryOnStorageInit retries the given function when the API server returns a 429 TooManyRequests
// error, which happens when storage is (re)initializing after CRD installation.
func (m *Migrator) retryOnStorageInit(ctx context.Context, fn func() error) error {
	backoff := wait.Backoff{
		Duration: retryBackoffInitial,
		Factor:   retryBackoffFactor,
		Cap:      retryBackoffMax,
		Steps:    retryBackoffSteps,
	}

	var lastErr error

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		lastErr = fn()
		if lastErr == nil {
			return true, nil
		}

		// Retry on 429 TooManyRequests (storage initializing) or 503 ServiceUnavailable
		if apierrors.IsTooManyRequests(lastErr) || apierrors.IsServiceUnavailable(lastErr) {
			m.logger.V(1).Info("Retrying", "error", lastErr)

			return false, nil
		}

		// Non-retryable error
		return false, lastErr
	})

	if wait.Interrupted(err) {
		return fmt.Errorf("timed out: %w", lastErr)
	}

	return err
}
