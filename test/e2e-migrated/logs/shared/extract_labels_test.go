package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
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
				return loggen.New(ns).WithLabels(labels).K8sObject()
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
				backendNs    = uniquePrefix("backend")

				genNs        = uniquePrefix("gen")
				pipelineName = uniquePrefix()
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
			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(genNs, genLabels),
			}
			resources = append(resources, backend.K8sObjects()...)

			Eventually(func(g Gomega) int {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Enrichments = &operatorv1alpha1.EnrichmentSpec{
					ExtractPodLabels: []operatorv1alpha1.PodLabel{
						{
							Key: "log.test.exact.should.match",
						},
						{
							KeyPrefix: "log.test.prefix",
						},
					},
				}
				err = suite.K8sClient.Update(t.Context(), &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return len(telemetry.Spec.Enrichments.ExtractPodLabels)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(2))

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), types.NamespacedName{Name: kitbackend.DefaultName, Namespace: backendNs})
			assert.OTelLogPipelineHealthy(t.Context(), pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t.Context(), backend, genNs)

			assert.BackendDataConsistentlyMatches(t.Context(), backend, HaveFlatLogs(
				HaveEach(SatisfyAll(
					HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyExactMatch, labelValueExactMatch)),
					HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch1, labelValuePrefixMatch1)),
					HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch2, labelValuePrefixMatch2)),
					Not(HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyShouldNotMatch, labelValueShouldNotMatch))),
				)),
			))
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

	logProducer := loggen.New(genNs).
		WithLabels(map[string]string{"env": "dev"}).
		WithAnnotations(map[string]string{"release": "v1.0.0"})

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

	assert.FluentBitLogPipelineHealthy(t.Context(), pipelineNameNotDropped)
	assert.FluentBitLogPipelineHealthy(t.Context(), pipelineNameDropped)
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), types.NamespacedName{Namespace: notDroppedNs, Name: kitbackend.DefaultName})
	assert.DeploymentReady(t.Context(), types.NamespacedName{Namespace: droppedNs, Name: kitbackend.DefaultName})
	assert.DeploymentReady(t.Context(), types.NamespacedName{Namespace: genNs, Name: loggen.DefaultName})

	// Scenario 1: Labels not dropped
	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backendNotDropped, genNs)
	assert.BackendDataEventuallyMatches(t.Context(), backendNotDropped, fluentbit.HaveFlatLogs(
		HaveEach(fluentbit.HaveKubernetesLabels(HaveKeyWithValue("env", "dev")))),
	)
	assert.BackendDataConsistentlyMatches(t.Context(), backendNotDropped, fluentbit.HaveFlatLogs(
		Not(HaveEach(
			fluentbit.HaveKubernetesAnnotations(Not(BeEmpty())),
		)),
	))

	// Scenario 2: Labels dropped

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backendDropped, genNs)
	assert.BackendDataConsistentlyMatches(t.Context(), backendDropped, fluentbit.HaveFlatLogs(
		HaveEach(Not(
			fluentbit.HaveKubernetesLabels(HaveKeyWithValue("env", "dev")),
		)),
	))
	assert.BackendDataConsistentlyMatches(t.Context(), backendDropped, fluentbit.HaveFlatLogs(
		Not(ContainElement(
			fluentbit.HaveKubernetesAnnotations(Not(BeEmpty())),
		)),
	))
}
