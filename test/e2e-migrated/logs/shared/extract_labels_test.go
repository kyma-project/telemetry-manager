package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestExtractLabels_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNs string) telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string, labels map[string]string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineApplicationInput(testutils.ExtIncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string, labels map[string]string) client.Object {
				return stdloggen.NewDeployment(ns).WithLabels(labels).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
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
				telemetry    operatorv1alpha1.Telemetry
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			genLabels := map[string]string{
				labelKeyExactMatch:     labelValueExactMatch,
				labelKeyPrefixMatch1:   labelValuePrefixMatch1,
				labelKeyPrefixMatch2:   labelValuePrefixMatch2,
				labelKeyShouldNotMatch: labelValueShouldNotMatch,
			}

			Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
			telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1alpha1.PodLabel{
					{Key: "log.test.exact.should.match"},
					{KeyPrefix: "log.test.prefix"},
				},
			}
			Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred(), "should update Telemetry resource with enrichment configuration")

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs, genLabels),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects

				Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).Should(Succeed())
				telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{}
				require.NoError(t, suite.K8sClient.Update(context.Background(), &telemetry)) //nolint:usetesting // Remove ctx from Update
			})
			Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)
			}

			assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), backend.NamespacedName())
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

	logProducer := stdloggen.NewDeployment(genNs).
		WithLabel("env", "dev").
		WithAnnotation("release", "v1.0.0")

	pipelineNotDropped := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameNotDropped).
		WithKeepAnnotations(false).
		WithApplicationInput(true, testutils.ExtIncludeNamespaces(genNs)).
		WithDropLabels(false).
		WithHTTPOutput(testutils.HTTPHost(backendNotDropped.Host()), testutils.HTTPPort(backendNotDropped.Port())).
		Build()

	pipelineDropped := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameDropped).
		WithKeepAnnotations(false).
		WithApplicationInput(true, testutils.ExtIncludeNamespaces(genNs)).
		WithDropLabels(true).
		WithHTTPOutput(testutils.HTTPHost(backendDropped.Host()), testutils.HTTPPort(backendDropped.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(notDroppedNs).K8sObject(),
		kitk8s.NewNamespace(droppedNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		logProducer.K8sObject(),
		&pipelineNotDropped,
		&pipelineDropped,
	}
	resources = append(resources, backendNotDropped.K8sObjects()...)
	resources = append(resources, backendDropped.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t, pipelineNameNotDropped)
	assert.FluentBitLogPipelineHealthy(t, pipelineNameDropped)
	assert.DaemonSetReady(t.Context(), kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), backendNotDropped.NamespacedName())
	assert.DeploymentReady(t.Context(), backendDropped.NamespacedName())
	assert.DeploymentReady(t.Context(), logProducer.NamespacedName())

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
