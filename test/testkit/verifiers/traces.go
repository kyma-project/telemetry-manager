package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func TracePipelineShouldBeHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeGatewayHealthy)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeRunning)).To(BeTrue())
		g.Expect(meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypePending)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TraceCollectorConfigShouldContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, kitkyma.TraceGatewayName, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return strings.Contains(configString, pipelineAlias)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue())
}

func TraceCollectorConfigShouldNotContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Consistently(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, kitkyma.TraceGatewayName, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return !strings.Contains(configString, pipelineAlias)
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(BeTrue())
}

func TracesFromNamespaceShouldBeDelivered(proxyClient *apiserverproxy.Client, telemetryExportURL, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			ContainTd(ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", namespace))),
		))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func TracesFromNamespacesShouldNotBeDelivered(proxyClient *apiserverproxy.Client, telemetryExportURL string, namespaces []string) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			Not(ContainTd(ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", BeElementOf(namespaces))))),
		))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
