package storagemigration

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
)

const (
	oldVersion = "v1alpha1"

	migrationTimeout = 5 * time.Minute
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

func (m *Migrator) Start(ctx context.Context) error {
	if err := m.migratePipelinesIfNeeded(ctx); err != nil {
		// Log the error but don't return it, as we don't want to crash the manager if migration fails.
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
	if err := m.client.Get(ctx, types.NamespacedName{Name: crdName}, &crd); err != nil {
		return false, fmt.Errorf("failed to get CRD %s: %w", crdName, err)
	}

	for _, version := range crd.Status.StoredVersions {
		metrics.MigratorInfo.WithLabelValues(crdName, version).Set(1)
	}

	return slices.Contains(crd.Status.StoredVersions, oldVersion), nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateLogPipelines(ctx context.Context) error {
	var list telemetryv1beta1.LogPipelineList
	if err := m.client.List(ctx, &list); err != nil {
		return fmt.Errorf("failed to list LogPipelines: %w", err)
	}

	m.logger.Info("Migrating LogPipelines", "count", len(list.Items))

	for i := range list.Items {
		if err := m.client.Update(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate LogPipeline %s: %w", list.Items[i].Name, err)
		}
	}

	return nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateMetricPipelines(ctx context.Context) error {
	var list telemetryv1beta1.MetricPipelineList
	if err := m.client.List(ctx, &list); err != nil {
		return fmt.Errorf("failed to list MetricPipelines: %w", err)
	}

	m.logger.Info("Migrating MetricPipelines", "count", len(list.Items))

	for i := range list.Items {
		if err := m.client.Update(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate MetricPipeline %s: %w", list.Items[i].Name, err)
		}
	}

	return nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateTracePipelines(ctx context.Context) error {
	var list telemetryv1beta1.TracePipelineList
	if err := m.client.List(ctx, &list); err != nil {
		return fmt.Errorf("failed to list TracePipelines: %w", err)
	}

	m.logger.Info("Migrating TracePipelines", "count", len(list.Items))

	for i := range list.Items {
		if err := m.client.Update(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate TracePipeline %s: %w", list.Items[i].Name, err)
		}
	}

	return nil
}

//nolint:dupl // similar structure for different types is intentional
func (m *Migrator) migrateTelemetries(ctx context.Context) error {
	var list operatorv1beta1.TelemetryList
	if err := m.client.List(ctx, &list); err != nil {
		return fmt.Errorf("failed to list Telemetries: %w", err)
	}

	m.logger.Info("Migrating Telemetries", "count", len(list.Items))

	for i := range list.Items {
		if err := m.client.Update(ctx, &list.Items[i]); err != nil {
			return fmt.Errorf("failed to migrate Telemetry %s/%s: %w", list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}

	return nil
}

func (m *Migrator) clearStoredVersion(ctx context.Context, crdName string) error {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := m.client.Get(ctx, types.NamespacedName{Name: crdName}, &crd); err != nil {
		return fmt.Errorf("failed to get CRD %s: %w", crdName, err)
	}

	newStoredVersions := make([]string, 0, len(crd.Status.StoredVersions))
	for _, version := range crd.Status.StoredVersions {
		if version != oldVersion {
			newStoredVersions = append(newStoredVersions, version)
		}
	}

	if len(newStoredVersions) == len(crd.Status.StoredVersions) {
		m.logger.Info("StoredVersions already clean", "crd", crdName)
		return nil
	}

	crd.Status.StoredVersions = newStoredVersions
	if err := m.client.Status().Update(ctx, &crd); err != nil {
		return fmt.Errorf("failed to update CRD status for %s: %w", crdName, err)
	}

	metrics.MigratorInfo.WithLabelValues(crdName, oldVersion).Set(0)

	for _, version := range newStoredVersions {
		metrics.MigratorInfo.WithLabelValues(crdName, version).Set(1)
	}

	m.logger.Info("Cleared old stored version", "crd", crdName, "removedVersion", oldVersion)

	return nil
}
