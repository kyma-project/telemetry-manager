package assert

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func TracesFromNamespaceDelivered(proxyClient *apiserverproxy.Client, backendExportURL, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			ContainTd(ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", namespace))),
		))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func TracesFromNamespacesNotDelivered(proxyClient *apiserverproxy.Client, backendExportURL string, namespaces []string) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			Not(ContainTd(ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", BeElementOf(namespaces))))),
		))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func TracePipelineHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())

		gatewayHealthy := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeGatewayHealthy)
		g.Expect(gatewayHealthy).NotTo(BeNil())
		g.Expect(gatewayHealthy.Status).To(BeTrueBecause("Gateway not healthy. Reason: %s. Message: %s", gatewayHealthy.Reason, gatewayHealthy.Message))

		configGenerated := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		g.Expect(configGenerated).NotTo(BeNil())
		g.Expect(configGenerated.Status).To(BeTrueBecause("Configuration not generated. Reason: %s. Message: %s", configGenerated.Reason, configGenerated.Message))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TracePipelineHasCondition(ctx context.Context, k8sClient client.Client, pipelineName string, expectedCond metav1.Condition) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		condition := meta.FindStatusCondition(pipeline.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineConditionReasonsTransition,TracePipelineConditionReasonsTransition, MetricPipelineConditionReasonsTransition have similarities, but they are not the same
func TracePipelineConditionReasonsTransition(ctx context.Context, k8sClient client.Client, pipelineName, condType string, expected []ReasonStatus) {
	var currCond *metav1.Condition

	for _, expected := range expected {
		// Wait for the current condition to match the expected condition
		Eventually(func(g Gomega) ReasonStatus {
			var pipeline telemetryv1alpha1.TracePipeline
			key := types.NamespacedName{Name: pipelineName}
			err := k8sClient.Get(ctx, key, &pipeline)
			g.Expect(err).To(Succeed())
			currCond = meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			if currCond == nil {
				return ReasonStatus{}
			}

			return ReasonStatus{Reason: currCond.Reason, Status: currCond.Status}
		}, 10*time.Minute, periodic.DefaultInterval).Should(Equal(expected), "expected reason %s[%s] of type %s not reached", expected.Reason, expected.Status, condType)

		fmt.Fprintf(GinkgoWriter, "Transitioned to [%s]%s\n", currCond.Status, currCond.Reason)
	}
}
