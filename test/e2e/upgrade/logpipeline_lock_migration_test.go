package upgrade

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestLogPipelineLockMigration verifies that upgrading from a version that used a single shared
// LogPipeline lock (for both OTel and FluentBit pipelines) to a version with separate locks cleans
// up the stale FluentBit owner reference from the old shared lock. Otherwise the phantom FluentBit
// owner would count against the OTel pipeline limit.
//
// The pinned version (1.70.0) predates the OTel/FluentBit lock split (PR #3822), so both pipeline
// types become owner references on telemetry-logpipeline-lock. After upgrade, the migration runnable
// must remove the FluentBit owner so the lock holds only OTel pipelines.
func TestLogPipelineLockMigration(t *testing.T) {
	labels := []string{suite.LabelMisc, suite.LabelTelemetry, suite.LabelUpgrade}
	suite.SetupTestWithOptions(t, labels,
		kubeprep.WithForceFreshInstall(),
		kubeprep.WithSkipDeployTestPrerequisites(),
		kubeprep.WithChartVersion("https://github.com/kyma-project/telemetry-manager/releases/download/1.70.0/telemetry-manager-1.70.0.tgz"),
	)

	var (
		uniquePrefix      = unique.Prefix("logpipeline-lock-migration")
		otelPipeline      = uniquePrefix("otel")
		fluentBitPipeline = uniquePrefix("fluentbit")
	)

	telemetry := operatorv1alpha1.Telemetry{
		Name:      "default",
		Namespace: "kyma-system",
	}

	Expect(kitk8s.CreateObjects(t, &telemetry)).To(Succeed())

	otelLP := telemetryv1alpha1.LogPipeline{
		Name: otelPipeline,
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{Value: "http://localhost:4317"},
				},
			},
		},
	}

	fluentBitLP := telemetryv1alpha1.LogPipeline{
		Name: fluentBitPipeline,
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				FluentBitCustom: "Name stdout",
			},
		},
	}

	Expect(kitk8s.CreateObjects(t, &otelLP, &fluentBitLP)).To(Succeed())

	// Before upgrade: the single shared lock should hold both pipelines as owners.
	Eventually(func(g Gomega) {
		lock := getSharedLock(g)
		g.Expect(lockOwnsPipeline(lock, otelLP.Name)).To(BeTrue(), "OTel pipeline should own the shared lock before upgrade")
		g.Expect(lockOwnsPipeline(lock, fluentBitLP.Name)).To(BeTrue(), "FluentBit pipeline should own the shared lock before upgrade")
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	Expect(suite.UpgradeToTargetVersion(t)).To(Succeed())

	// After upgrade: the FluentBit owner must be gone from the (rebuilt) shared lock, the OTel owner remains.
	Eventually(func(g Gomega) {
		lock := getSharedLock(g)
		g.Expect(lockOwnsPipeline(lock, fluentBitLP.Name)).To(BeFalse(), "FluentBit owner should be cleaned up from the shared lock after upgrade")
		g.Expect(lockOwnsPipeline(lock, otelLP.Name)).To(BeTrue(), "OTel pipeline should still own the shared lock after upgrade")
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	// Both pipelines should survive the upgrade.
	Expect(suite.K8sClient.Get(suite.Ctx, client.ObjectKeyFromObject(&otelLP), &otelLP)).To(Succeed())
	Expect(suite.K8sClient.Get(suite.Ctx, client.ObjectKeyFromObject(&fluentBitLP), &fluentBitLP)).To(Succeed())
}

func getSharedLock(g Gomega) *corev1.ConfigMap {
	var lock corev1.ConfigMap
	g.Expect(suite.K8sClient.Get(suite.Ctx, types.NamespacedName{Name: names.LogPipelineLock, Namespace: "kyma-system"}, &lock)).To(Succeed())

	return &lock
}

func lockOwnsPipeline(lock *corev1.ConfigMap, pipelineName string) bool {
	for _, ref := range lock.GetOwnerReferences() {
		if ref.Kind == "LogPipeline" && ref.Name == pipelineName {
			return true
		}
	}

	return false
}
