package kubeprep

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

// runtimeResourceSelector selects resources created at runtime by the telemetry-manager.
// Helm chart resources use "app.kubernetes.io/managed-by: kyma", while the manager
// stamps its runtime-created resources with "app.kubernetes.io/managed-by: telemetry-manager".
var runtimeResourceSelector = client.MatchingLabels{
	commonresources.LabelKeyK8sManagedBy: commonresources.LabelValueK8sManagedBy,
}

// WaitForManagedResourceCleanup waits until all runtime-created resources
// (labeled with app.kubernetes.io/managed-by=telemetry-manager) are deleted
// from the kyma-system namespace.
//
// This is intended to be called from t.Cleanup after pipeline resources have been
// deleted, giving the manager time to reconcile and remove dependent resources
// like collectors, agents, and the selfmonitor.
func WaitForManagedResourceCleanup(ctx context.Context, k8sClient client.Client) {
	gomega.Eventually(allRuntimeResourcesDeleted(ctx, k8sClient), periodic.EventuallyTimeout, periodic.DefaultInterval).Should(gomega.Succeed())
}

func allRuntimeResourcesDeleted(ctx context.Context, k8sClient client.Client) func() error {
	ns := client.InNamespace(kymaSystemNamespace)

	return func() error {
		var deployments appsv1.DeploymentList
		if err := k8sClient.List(ctx, &deployments, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		if n := len(deployments.Items); n > 0 {
			return fmt.Errorf("%d deployment(s) still exist", n)
		}

		var daemonSets appsv1.DaemonSetList
		if err := k8sClient.List(ctx, &daemonSets, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list daemonsets: %w", err)
		}

		if n := len(daemonSets.Items); n > 0 {
			return fmt.Errorf("%d daemonset(s) still exist", n)
		}

		var configMaps corev1.ConfigMapList
		if err := k8sClient.List(ctx, &configMaps, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list configmaps: %w", err)
		}

		if n := len(configMaps.Items); n > 0 {
			return fmt.Errorf("%d configmap(s) still exist", n)
		}

		var secrets corev1.SecretList
		if err := k8sClient.List(ctx, &secrets, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}

		if n := len(secrets.Items); n > 0 {
			return fmt.Errorf("%d secret(s) still exist", n)
		}

		return nil
	}
}
