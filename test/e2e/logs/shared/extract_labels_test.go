package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestExtractLabels_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNs string) telemetryv1beta1.LogPipelineInput
		logGeneratorBuilder func(ns string, labels map[string]string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string, labels map[string]string) client.Object {
				return stdoutloggen.NewDeployment(ns).WithLabels(labels).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string, labels map[string]string) client.Object {
				return telemetrygen.NewPod(ns, telemetrygen.SignalTypeLogs).WithLabels(labels).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const (
				k8sLabelKeyPrefix = "k8s.pod.label"
				logLabelKeyPrefix = "log.test.prefix"

				labelKeyExactMatch     = "log.test.exact.should.match"
				labelKeyPrefixMatch1   = logLabelKeyPrefix + ".should.match1"
				labelKeyPrefixMatch2   = logLabelKeyPrefix + ".should.match2"
				labelKeyShouldNotMatch = "log.test.label.should.not.match"

				labelValueExactMatch     = "exact_match"
				labelValuePrefixMatch1   = "prefix_match1"
				labelValuePrefixMatch2   = "prefix_match2"
				labelValueShouldNotMatch = "should_not_match"
			)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
				telemetry    operatorv1beta1.Telemetry
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			genLabels := map[string]string{
				labelKeyExactMatch:     labelValueExactMatch,
				labelKeyPrefixMatch1:   labelValuePrefixMatch1,
				labelKeyPrefixMatch2:   labelValuePrefixMatch2,
				labelKeyShouldNotMatch: labelValueShouldNotMatch,
			}

			kitk8s.PreserveAndScheduleRestoreOfTelemetryResource(t, kitkyma.TelemetryName)

			Eventually(func(g Gomega) {
				g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
				telemetry.Spec.Enrichments = &operatorv1beta1.EnrichmentSpec{
					ExtractPodLabels: []operatorv1beta1.PodLabel{
						{Key: "log.test.exact.should.match"},
						{KeyPrefix: "log.test.prefix"},
					},
				}
				g.Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred(), "should update Telemetry resource with enrichment configuration")
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs, genLabels),
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
			}

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.LogGatewayName)
			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

			// Verify that at least one log entry contains the expected labels, rather than requiring all entries to match.
			// This approach accounts for potential delays in the k8sattributes processor syncing with the API server during startup,
			// which can result in some logs not being enriched and causing test flakiness.
			assert.BackendDataEventuallyMatches(t, backend,
				HaveFlatLogs(ContainElement(
					HaveResourceAttributes(SatisfyAll(
						HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyExactMatch, labelValueExactMatch),
						HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch1, labelValuePrefixMatch1),
						HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch2, labelValuePrefixMatch2),
						Not(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyShouldNotMatch, labelValueShouldNotMatch)),
					)),
				)),
			)
		})
	}
}

func TestExtractLabels_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix           = unique.Prefix()
		pipelineNameNotDropped = uniquePrefix("not-dropped")
		pipelineNameDropped    = uniquePrefix("dropped")
		notDroppedNs           = uniquePrefix("not-dropped")
		droppedNs              = uniquePrefix("dropped")
		genNs                  = uniquePrefix("gen")
	)

	backendNotDropped := kitbackend.New(notDroppedNs, kitbackend.SignalTypeLogsFluentBit)
	backendDropped := kitbackend.New(droppedNs, kitbackend.SignalTypeLogsFluentBit)

	logProducer := stdoutloggen.NewDeployment(genNs).
		WithLabel("env", "dev").
		WithAnnotation("release", "v1.0.0")

	pipelineNotDropped := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameNotDropped).
		WithKeepAnnotations(false).
		WithRuntimeInput(true, testutils.IncludeNamespaces(genNs)).
		WithDropLabels(false).
		WithHTTPOutput(testutils.HTTPHost(backendNotDropped.Host()), testutils.HTTPPort(backendNotDropped.Port())).
		Build()

	pipelineDropped := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameDropped).
		WithKeepAnnotations(false).
		WithRuntimeInput(true, testutils.IncludeNamespaces(genNs)).
		WithDropLabels(true).
		WithHTTPOutput(testutils.HTTPHost(backendDropped.Host()), testutils.HTTPPort(backendDropped.Port())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(notDroppedNs).K8sObject(),
		kitk8sobjects.NewNamespace(droppedNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		logProducer.K8sObject(),
		&pipelineNotDropped,
		&pipelineDropped,
	}
	resources = append(resources, backendNotDropped.K8sObjects()...)
	resources = append(resources, backendDropped.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backendNotDropped)
	assert.BackendReachable(t, backendDropped)
	assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t, logProducer.NamespacedName())
	assert.FluentBitLogPipelineHealthy(t, pipelineNameNotDropped)
	assert.FluentBitLogPipelineHealthy(t, pipelineNameDropped)

	// Scenario 1: Labels not dropped
	assert.FluentBitLogsFromNamespaceDelivered(t, backendNotDropped, genNs)
	assert.BackendDataEventuallyMatches(t, backendNotDropped, fluentbit.HaveFlatLogs(
		HaveEach(fluentbit.HaveKubernetesLabels(HaveKeyWithValue("env", "dev")))),
	)
	assert.BackendDataConsistentlyMatches(t, backendNotDropped, fluentbit.HaveFlatLogs(
		Not(HaveEach(
			fluentbit.HaveKubernetesAnnotations(Not(BeEmpty())),
		)),
	))

	// Scenario 2: Labels dropped

	assert.FluentBitLogsFromNamespaceDelivered(t, backendDropped, genNs)
	assert.BackendDataConsistentlyMatches(t, backendDropped, fluentbit.HaveFlatLogs(
		HaveEach(Not(
			fluentbit.HaveKubernetesLabels(HaveKeyWithValue("env", "dev")),
		)),
	))
	assert.BackendDataConsistentlyMatches(t, backendDropped, fluentbit.HaveFlatLogs(
		Not(ContainElement(
			fluentbit.HaveKubernetesAnnotations(Not(BeEmpty())),
		)),
	))
}
