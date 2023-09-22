package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
)

func MetricPipelineShouldBeRunning(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) bool {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		return pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning)
	}, timeout, interval).Should(BeTrue())
}

func MetricPipelineShouldStayPending(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Consistently(func(g Gomega) {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning)).To(BeFalse())
	}, reconciliationTimeout, interval).Should(Succeed())
}

func MetricGatewayConfigShouldContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		metricGatewayName
		g.Expect(k8sClient.Get(ctx, key, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return strings.Contains(configString, pipelineAlias)
	}, timeout, interval).Should(BeTrue())
}

func MetricGatewayConfigShouldNotContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Consistently(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		metricGatewayName
		g.Expect(k8sClient.Get(ctx, key, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return !strings.Contains(configString, pipelineAlias)
	}, reconciliationTimeout, interval).Should(BeTrue())
}

func MetricsShouldBeDelivered(proxyClient *apiserver.ProxyClient, proxyURL string, metrics []pmetric.Metric) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ConsistOfMds(WithMetrics(BeEquivalentTo(metrics)))))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, timeout, interval).Should(Succeed())
}
