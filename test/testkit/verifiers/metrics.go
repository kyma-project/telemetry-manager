package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func MetricPipelineShouldBeRunning(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Eventually(func(g gomega.Gomega) bool {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(gomega.Succeed())
		return pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(gomega.BeTrue())
}

func MetricPipelineShouldNotBeRunningPending(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Consistently(func(g gomega.Gomega) {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(gomega.Succeed())
		g.Expect(pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning)).To(gomega.BeFalse())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(gomega.Succeed())
}

func MetricGatewayConfigShouldContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Eventually(func(g gomega.Gomega) bool {
		var collectorConfig corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, kitkyma.MetricGatewayName, &collectorConfig)).To(gomega.Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return strings.Contains(configString, pipelineAlias)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(gomega.BeTrue())
}

func MetricGatewayConfigShouldNotContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Consistently(func(g gomega.Gomega) bool {
		var collectorConfig corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, kitkyma.MetricGatewayName, &collectorConfig)).To(gomega.Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return !strings.Contains(configString, pipelineAlias)
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(gomega.BeTrue())
}

func MetricsShouldBeDelivered(proxyClient *apiserver.ProxyClient, telemetryExportURL string, metrics []pmetric.Metric) {
	gomega.Eventually(func(g gomega.Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(resp).To(gomega.HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(gomega.HaveHTTPBody(metric.ConsistOfMds(metric.WithMetrics(gomega.BeEquivalentTo(metrics)))))
		err = resp.Body.Close()
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(gomega.Succeed())
}
