package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

func TracePipelineShouldBeRunning(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Eventually(func(g gomega.Gomega) bool {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(gomega.Succeed())
		return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
	}, timeout, interval).Should(gomega.BeTrue())
}

func TracePipelineShouldStayPending(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Consistently(func(g gomega.Gomega) {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(gomega.Succeed())
		g.Expect(pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)).To(gomega.BeFalse())
	}, reconciliationTimeout, interval).Should(gomega.Succeed())
}

func TraceCollectorConfigShouldContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Eventually(func(g gomega.Gomega) bool {
		var collectorConfig corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, kitkyma.TraceGatewayName, &collectorConfig)).To(gomega.Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return strings.Contains(configString, pipelineAlias)
	}, timeout, interval).Should(gomega.BeTrue())
}

func TraceCollectorConfigShouldNotContainPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	gomega.Consistently(func(g gomega.Gomega) bool {
		var collectorConfig corev1.ConfigMap
		g.Expect(k8sClient.Get(ctx, kitkyma.TraceGatewayName, &collectorConfig)).To(gomega.Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return !strings.Contains(configString, pipelineAlias)
	}, reconciliationTimeout, interval).Should(gomega.BeTrue())
}

func TracesShouldBeDelivered(proxyClient *apiserver.ProxyClient, telemetryExportURL string, traceID pcommon.TraceID, spanIDs []pcommon.SpanID, attrs pcommon.Map) {
	gomega.Eventually(func(g gomega.Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(resp).To(gomega.HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(gomega.HaveHTTPBody(gomega.SatisfyAll(
			matchers.ConsistOfNumberOfSpans(len(spanIDs)),
			matchers.ConsistOfSpansWithIDs(spanIDs...),
			matchers.ConsistOfSpansWithTraceID(traceID),
			matchers.ConsistOfSpansWithAttributes(attrs))))
		err = resp.Body.Close()
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}, timeout, interval).Should(gomega.Succeed())
}
