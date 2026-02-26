package istio

import (
	"fmt"
	"log"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	"github.com/kyma-project/telemetry-manager/test/testkit/istio"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestAccessLogsOTLP(t *testing.T) {
	tests := []struct {
		name               string
		labels             []string
		opts               []kubeprep.Option
		resourceName       types.NamespacedName
		readinessCheckFunc func(t *testing.T, name types.NamespacedName)
	}{

		{
			name:               suite.LabelIstio,
			labels:             []string{suite.LabelGardener},
			opts:               []kubeprep.Option{kubeprep.WithIstio()},
			resourceName:       kitkyma.LogGatewayName,
			readinessCheckFunc: assert.DeploymentReady,
		},
		{
			name:               fmt.Sprintf("%s-%s", suite.LabelIstio, suite.LabelExperimental),
			labels:             []string{},
			opts:               []kubeprep.Option{kubeprep.WithIstio(), kubeprep.WithExperimental()},
			resourceName:       kitkyma.TelemetryOTLPGatewayName,
			readinessCheckFunc: assert.DaemonSetReady,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log.Printf("running test with labels: %v", tc.labels)
			suite.SetupTestWithOptions(t, tc.labels, tc.opts...)
			log.Printf("registered test case with labels: %v", tc.labels)
			log.Printf("running test with resource: %v", tc.resourceName)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
			)

			logBackend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel, kitbackend.WithName(uniquePrefix("access-logs")))
			traceBackend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces, kitbackend.WithName(uniquePrefix("traces")))

			logPipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithRuntimeInput(false).
				WithOTLPOutput(testutils.OTLPEndpoint(logBackend.EndpointHTTP())).
				Build()

			sampleApp := prommetricgen.New(permissiveNs, prommetricgen.WithName(uniquePrefix("otlp-access-log-emitter")))
			metricPodURL := suite.ProxyClient.ProxyURLForPod(permissiveNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort())

			tracePipeline := testutils.NewTracePipelineBuilder().
				WithName(pipelineName).
				WithOTLPOutput(testutils.OTLPEndpoint(traceBackend.EndpointHTTP())).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
				&logPipeline,
				&tracePipeline,
				sampleApp.Pod().K8sObject(),
			}
			resources = append(resources, logBackend.K8sObjects()...)
			resources = append(resources, traceBackend.K8sObjects()...)

			require.NoError(t, kitk8s.CreateObjects(t, resources...))

			assert.BackendReachable(t, logBackend)
			assert.BackendReachable(t, traceBackend)

			listOptions := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "metric-producer"}),
				Namespace:     permissiveNs,
			}
			assert.PodsReady(t, listOptions)
			assert.OTelLogPipelineHealthy(t, pipelineName)
			tc.readinessCheckFunc(t, tc.resourceName)

			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(metricPodURL)
				g.Expect(err).NotTo(HaveOccurred())

				defer resp.Body.Close()

				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed(),
				"Should invoke the metrics endpoint to generate access logs",
			)

			assert.BackendDataEventuallyMatches(t, logBackend,
				HaveFlatLogs(ContainElement(SatisfyAll(
					HaveAttributes(HaveKey(BeElementOf(istio.AccessLogOTLPLogAttributeKeys))),
					HaveSeverityNumber(Equal(9)),
					HaveSeverityText(Equal("INFO")),
					HaveScopeName(Equal("io.kyma-project.telemetry/istio")),
					HaveScopeVersion(SatisfyAny(
						Equal("main"),
						MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
					)),
				))), assert.WithOptionalDescription("Istio OTLP access logs should be present"),
			)

			assert.BackendDataConsistentlyMatches(t, logBackend,
				HaveFlatLogs(ContainElement(SatisfyAll(
					Not(HaveResourceAttributes(HaveKey("cluster_name"))),
					Not(HaveResourceAttributes(HaveKey("log_name"))),
					Not(HaveResourceAttributes(HaveKey("zone_name"))),
					Not(HaveResourceAttributes(HaveKey("node_name"))),
					Not(HaveAttributes(HaveKey("kyma.module"))),
				))), assert.WithOptionalDescription("Istio cluster attributes should not be present"),
			)

			assert.BackendDataConsistentlyMatches(t, logBackend,
				HaveFlatLogs(Not(ContainElement(SatisfyAny(
					HaveResourceAttributes(HaveKeyWithValue("k8s.deployment.name", "telemetry-otlp-traces")),
					HaveAttributes(HaveKeyWithValue("server.address", "telemetry-otlp-traces.kyma-system:4317")),
				)))), assert.WithOptionalDescription("Istio noise filter should be applied"),
			)
		})
	}
}
