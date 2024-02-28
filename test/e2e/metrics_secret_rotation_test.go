//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Secret Rotation", Label("metrics"), func() {
	Context("When a metricpipeline with missing secret reference exists", Ordered, func() {
		hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", kitkyma.DefaultNamespaceName,
			kitk8s.WithStringData("metric-host", "http://localhost:4317"))
		metricPipeline := kitk8s.NewMetricPipeline("without-secret").WithOutputEndpointFromSecret(hostSecret.SecretKeyRef("metric-host"))

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipeline.K8sObject())).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, metricPipeline.K8sObject(), hostSecret.K8sObject())).Should(Succeed())
			})
		})

		It("Should set ConfigurationGenerated condition to false", func() {
			Eventually(func(g Gomega) {
				var fetched telemetryv1alpha1.MetricPipeline
				key := types.NamespacedName{Name: metricPipeline.Name()}
				g.Expect(k8sClient.Get(ctx, key, &fetched)).To(Succeed())
				configurationGeneratedCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeConfigurationGenerated)
				g.Expect(configurationGeneratedCond).NotTo(BeNil())
				g.Expect(configurationGeneratedCond.Status).Should(Equal(metav1.ConditionFalse))
				g.Expect(configurationGeneratedCond.Reason).Should(Equal(conditions.ReasonReferencedSecretMissing))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not have metric gateway deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.MetricGatewayName, &deployment)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue())
		})

		It("Should have running metricpipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, hostSecret.K8sObject())).Should(Succeed())
			})

			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, metricPipeline.Name())
		})
	})
})
