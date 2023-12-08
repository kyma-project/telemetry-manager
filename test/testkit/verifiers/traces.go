package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func TracePipelineShouldBeRunning(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) bool {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue())
}

func TracePipelineShouldNotBeRunning(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Consistently(func(g Gomega) {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)).To(BeFalse())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
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

func TracesShouldBeDelivered(proxyClient *apiserver.ProxyClient, telemetryExportURL string,
	traceID pcommon.TraceID,
	spanIDs []pcommon.SpanID,
	spanAttrs pcommon.Map) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ConsistOfTds(
			WithSpans(
				SatisfyAll(
					HaveLen(len(spanIDs)),
					WithSpanIDs(ConsistOf(spanIDs)),
					HaveEach(SatisfyAll(
						WithTraceID(Equal(traceID)),
						WithSpanAttrs(BeEquivalentTo(spanAttrs.AsRaw())),
					)),
				),
			))))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func TracesShouldNotBePresent(proxyClient *apiserver.ProxyClient, telemetryExportURL string, traceID pcommon.TraceID) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(telemetryExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(Not(ContainTd(
			ContainSpan(WithTraceID(Equal(traceID))),
		))))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
