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
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestExtractLabels_OTel(t *testing.T) {
	tests := []struct {
		label        string
		inputBuilder func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput
		expectAgent  bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
				var opts []testutils.ExtendedNamespaceSelectorOptions
				if includeNs != "" {
					opts = append(opts, testutils.ExtIncludeNamespaces(includeNs))
				}
				if excludeNs != "" {
					opts = append(opts, testutils.ExtExcludeNamespaces(excludeNs))
				}

				return testutils.BuildLogPipelineApplicationInput(opts...)
			},
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs, excludeNs string) telemetryv1alpha1.LogPipelineInput {
				var opts []testutils.ExtendedNamespaceSelectorOptions
				if includeNs != "" {
					opts = append(opts, testutils.ExtIncludeNamespaces(includeNs))
				}
				if excludeNs != "" {
					opts = append(opts, testutils.ExtExcludeNamespaces(excludeNs))
				}

				return testutils.BuildLogPipelineApplicationInput(opts...)
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
				pipelineName = uniquePrefix(tc.label)
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
			backendExportURL := backend.ExportURL(suite.ProxyClient)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs, "")).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			if tc.expectAgent {
				resources = append(resources, loggen.New(genNs).
					WithLabels(map[string]string{
						labelKeyExactMatch:     labelValueExactMatch,
						labelKeyPrefixMatch1:   labelValuePrefixMatch1,
						labelKeyPrefixMatch2:   labelValuePrefixMatch2,
						labelKeyShouldNotMatch: labelValueShouldNotMatch,
					}).
					K8sObject(),
				)
			} else {
				resources = append(resources, telemetrygen.NewPod(genNs, telemetrygen.SignalTypeLogs).
					WithLabel(labelKeyExactMatch, labelValueExactMatch).
					WithLabel(labelKeyPrefixMatch1, labelValuePrefixMatch1).
					WithLabel(labelKeyPrefixMatch2, labelValuePrefixMatch2).
					WithLabel(labelKeyShouldNotMatch, labelValueShouldNotMatch).
					K8sObject(),
				)
			}

			Eventually(func(g Gomega) int {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				// TODO: After Hisar's merge => API changed to telemetry.Spec instead of telemetry.Spec.Log => modify this

				telemetry.Spec.Log = &operatorv1alpha1.LogSpec{
					Enrichments: &operatorv1alpha1.EnrichmentSpec{
						Enabled: true,
						ExtractPodLabels: []operatorv1alpha1.PodLabel{
							{
								Key: "log.test.exact.should.match",
							},
							{
								KeyPrefix: "log.test.prefix",
							},
						},
					},
				}
				err = suite.K8sClient.Update(t.Context(), &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return len(telemetry.Spec.Log.Enrichments.ExtractPodLabels)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(2))

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: backendNs})
			assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, genNs)

			assert.DataConsistentlyMatching(suite.ProxyClient, backendExportURL, HaveFlatOTelLogs(
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
		genNs                  = uniquePrefix("generator")
	)

	backendNotDropped := kitbackend.New(notDroppedNs, kitbackend.SignalTypeLogsFluentBit)
	backendNotDroppedExportURL := backendNotDropped.ExportURL(suite.ProxyClient)

	backendDropped := kitbackend.New(droppedNs, kitbackend.SignalTypeLogsFluentBit)
	backendDroppedExportURL := backendDropped.ExportURL(suite.ProxyClient)

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
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineNameNotDropped)
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineNameDropped)
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Namespace: notDroppedNs, Name: kitbackend.DefaultName})
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Namespace: droppedNs, Name: kitbackend.DefaultName})
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Namespace: genNs, Name: loggen.DefaultName})

	// Scenario 1: Labels not dropped
	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendNotDroppedExportURL, genNs)
	assert.DataEventuallyMatching(suite.ProxyClient, backendNotDroppedExportURL, HaveFlatFluentBitLogs(
		HaveEach(HaveKubernetesLabels(HaveKeyWithValue("env", "dev")))),
	)

	assert.DataConsistentlyMatching(suite.ProxyClient, backendNotDroppedExportURL, HaveFlatFluentBitLogs(
		Not(HaveEach(
			HaveKubernetesAnnotations(Not(BeEmpty())),
		)),
	))

	// Scenario 2: Labels dropped

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendDroppedExportURL, genNs)
	assert.DataConsistentlyMatching(suite.ProxyClient, backendDroppedExportURL, HaveFlatFluentBitLogs(
		HaveEach(Not(
			HaveKubernetesLabels(HaveKeyWithValue("env", "dev")),
		)),
	))

	assert.DataConsistentlyMatching(suite.ProxyClient, backendDroppedExportURL, HaveFlatFluentBitLogs(
		Not(ContainElement(
			HaveKubernetesAnnotations(Not(BeEmpty())),
		)),
	))
}
