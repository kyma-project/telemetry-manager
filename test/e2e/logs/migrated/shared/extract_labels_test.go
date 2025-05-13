package shared

import (
	"context"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
	RegisterTestingT(t)

	tests := []struct {
		name         string
		inputBuilder func() telemetryv1alpha1.LogPipelineInput
	}{
		{
			name: "gateway",
			inputBuilder: func() telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{
						Enabled: ptr.To(false),
					},
				}
			},
		},
		{
			name: "agent", // FIXME: Currently failing (Label Extraction not implemented for OTel Agent)
			inputBuilder: func() telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{
						Enabled: ptr.To(true),
					},
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
				uniquePrefix = unique.Prefix()
				backendNs    = uniquePrefix("backend")
				generatorNs  = uniquePrefix("generator")
				pipelineName = uniquePrefix(tc.name)
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
			backendExportURL := backend.ExportURL(suite.ProxyClient)
			hostSecretRef := backend.HostSecretRefV1Alpha1()

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder()).
				WithOTLPOutput(
					testutils.OTLPEndpointFromSecret(
						hostSecretRef.Name,
						hostSecretRef.Namespace,
						hostSecretRef.Key,
					),
				).
				Build()

			otlpLogGen := telemetrygen.NewPod(generatorNs, telemetrygen.SignalTypeLogs).
				WithLabel(labelKeyExactMatch, labelValueExactMatch).
				WithLabel(labelKeyPrefixMatch1, labelValuePrefixMatch1).
				WithLabel(labelKeyPrefixMatch2, labelValuePrefixMatch2).
				WithLabel("log.test.label.should.not.match", "should_not_match").
				K8sObject()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(generatorNs).K8sObject(),
				otlpLogGen,
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			Eventually(func(g Gomega) int {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

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

			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)
			assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: backendNs})
			assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)

			Consistently(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatOTelLogs(ContainElement(SatisfyAll(
					HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyExactMatch, labelValueExactMatch)),
					HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch1, labelValuePrefixMatch1)),
					HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyPrefixMatch2, labelValuePrefixMatch2)),
					Not(HaveResourceAttributes(HaveKeyWithValue(k8sLabelKeyPrefix+"."+labelKeyShouldNotMatch, labelValueShouldNotMatch))),
				))))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	}
}

func TestExtractLabels_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix           = unique.Prefix()
		pipelineNameNotDropped = uniquePrefix("not-dropped")
		pipelineNameDropped    = uniquePrefix("dropped")
		notDroppedNs           = uniquePrefix("not-dropped")
		droppedNs              = uniquePrefix("dropped")
		generatorNs            = uniquePrefix("generator")
	)

	backendNotDropped := kitbackend.New(notDroppedNs, kitbackend.SignalTypeLogsFluentBit)
	backendNotDroppedExportURL := backendNotDropped.ExportURL(suite.ProxyClient)

	backendDropped := kitbackend.New(droppedNs, kitbackend.SignalTypeLogsFluentBit)
	backendDroppedExportURL := backendDropped.ExportURL(suite.ProxyClient)

	logProducer := loggen.New(generatorNs).
		WithLabels(map[string]string{"env": "dev"}).
		WithAnnotations(map[string]string{"release": "v1.0.0"})

	logPipelineNotDropped := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameNotDropped).
		WithKeepAnnotations(false).
		WithDropLabels(false).
		WithHTTPOutput(testutils.HTTPHost(backendNotDropped.Host()), testutils.HTTPPort(backendNotDropped.Port())).
		Build()

	logPipelineDropped := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameDropped).
		WithKeepAnnotations(false).
		WithDropLabels(true).
		WithHTTPOutput(testutils.HTTPHost(backendDropped.Host()), testutils.HTTPPort(backendDropped.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(notDroppedNs).K8sObject(),
		kitk8s.NewNamespace(droppedNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		logProducer.K8sObject(),
		&logPipelineNotDropped,
		&logPipelineDropped,
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
	assert.DeploymentReady(t.Context(), suite.K8sClient, types.NamespacedName{Namespace: generatorNs, Name: loggen.DefaultName})

	// Scenario 1: Labels not dropped
	assert.TelemetryDataDelivered(suite.ProxyClient, backendNotDroppedExportURL, HaveFlatFluentBitLogs(
		ContainElement(HaveKubernetesLabels(HaveKeyWithValue("env", "dev")))),
	)

	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendNotDroppedExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(HaveFlatFluentBitLogs(Not(ContainElement(
			HaveKubernetesAnnotations(Not(BeEmpty()))))),
		)
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())

	// Scenario 2: Labels dropped
	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendDroppedExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(Not(HaveHTTPBody(HaveFlatFluentBitLogs(
			ContainElement(HaveKubernetesLabels(HaveKeyWithValue("env", "dev")))),
		)))
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())

	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendDroppedExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(HaveFlatFluentBitLogs(Not(ContainElement(
			HaveKubernetesAnnotations(Not(BeEmpty()))))),
		)
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
