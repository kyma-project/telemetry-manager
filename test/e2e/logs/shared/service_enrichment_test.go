package shared

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestServiceEnrichment_OTel(t *testing.T) {
	tests := []struct {
		label        string
		inputBuilder func(includeNs string) telemetryv1beta1.LogPipelineInput
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const (
				// pod names
				podWithNoAnnotationsName           = "pod-with-no-annotations"
				podWithEmptyServiceAttributesName  = "pod-with-empty-service"
				podWithUnknownServiceName          = "pod-with-unknown-service"
				podWithUnknownServicePatternName   = "pod-with-unknown-service-pattern"
				podWithCustomServiceAttributesName = "pod-with-custom-service"

				// misc
				unknownService              = "unknown_service"
				unknownServicePattern       = "unknown_service:bash"
				annotationServiceName       = "resource.opentelemetry.io/service.name"
				annotationServiceNamespace  = "resource.opentelemetry.io/service.namespace"
				annotationServiceVersion    = "resource.opentelemetry.io/service.version"
				annotationServiceInstanceID = "resource.opentelemetry.io/service.instance.id"
				customServiceName           = "custom-service"
				customServiceNamespace      = "custom-namespace"
				customServiceVersion        = "v1.2.3"
				customServiceInstanceID     = "instance-1234"
			)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")

				telemetry operatorv1beta1.Telemetry
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)
			hostSecretRef := backend.HostSecretRefV1Beta1()

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithKeepOriginalBody(suite.ExpectAgent(tc.label)).
				WithOTLPOutput(
					testutils.OTLPEndpointFromSecret(
						hostSecretRef.Name,
						hostSecretRef.Namespace,
						hostSecretRef.Key,
					),
				).
				Build()

			// Enable OTel service enrichment strategy
			// TODO(TeodorSAP): Remove this block after deprecation period ends and OTel strategy becomes default enrichment strategy
			kitk8s.PreserveAndScheduleRestoreOfTelemetryResource(t, kitkyma.TelemetryName)
			Eventually(func(g Gomega) {
				g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetry)).NotTo(HaveOccurred())
				telemetry.Annotations = map[string]string{
					commonresources.AnnotationKeyTelemetryServiceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
				}
				g.Expect(suite.K8sClient.Update(t.Context(), &telemetry)).NotTo(HaveOccurred(), "should update Telemetry resource with service enrichment annotation")
			}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			if suite.ExpectAgent(tc.label) {
				// Configure generator pods for agent test
				podSpecLogs := stdoutloggen.PodSpec()

				resources = append(resources,
					kitk8sobjects.NewPod(podWithNoAnnotationsName, genNs).WithPodSpec(podSpecLogs).K8sObject(),
					kitk8sobjects.NewPod(podWithEmptyServiceAttributesName, genNs).
						WithAnnotation(annotationServiceName, "").
						WithAnnotation(annotationServiceNamespace, "").
						WithAnnotation(annotationServiceVersion, "").
						WithAnnotation(annotationServiceInstanceID, "").
						WithPodSpec(podSpecLogs).K8sObject(),
					kitk8sobjects.NewPod(podWithCustomServiceAttributesName, genNs).
						WithAnnotation(annotationServiceName, customServiceName).
						WithAnnotation(annotationServiceNamespace, customServiceNamespace).
						WithAnnotation(annotationServiceVersion, customServiceVersion).
						WithAnnotation(annotationServiceInstanceID, customServiceInstanceID).
						WithPodSpec(podSpecLogs).K8sObject(),
				)
			} else {
				// Configure generator pods for gateway test
				podSpecWithEmptyServiceAttributes := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs,
					telemetrygen.WithServiceName(""),
					telemetrygen.WithServiceNamespace(""),
					telemetrygen.WithServiceVersion(""),
					telemetrygen.WithServiceInstanceID(""),
				)
				podSpecWithUnknownServiceName := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs,
					telemetrygen.WithServiceName(unknownService))
				podSpecWithUnknownServiceNamePattern := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs,
					telemetrygen.WithServiceName(unknownServicePattern))
				podSpecWithCustomServiceAttributes := telemetrygen.PodSpec(telemetrygen.SignalTypeLogs,
					telemetrygen.WithServiceName(customServiceName),
					telemetrygen.WithServiceNamespace(customServiceNamespace),
					telemetrygen.WithServiceVersion(customServiceVersion),
					telemetrygen.WithServiceInstanceID(customServiceInstanceID),
				)

				resources = append(resources,
					kitk8sobjects.NewPod(podWithEmptyServiceAttributesName, genNs).WithPodSpec(podSpecWithEmptyServiceAttributes).K8sObject(),
					kitk8sobjects.NewPod(podWithUnknownServiceName, genNs).WithPodSpec(podSpecWithUnknownServiceName).K8sObject(),
					kitk8sobjects.NewPod(podWithUnknownServicePatternName, genNs).WithPodSpec(podSpecWithUnknownServiceNamePattern).K8sObject(),
					kitk8sobjects.NewPod(podWithCustomServiceAttributesName, genNs).WithPodSpec(podSpecWithCustomServiceAttributes).K8sObject(),
				)
			}

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.DeploymentReady(t, kitkyma.LogGatewayName)

			if suite.ExpectAgent(tc.label) {
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
			}

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.LogGatewayName)
			assert.OTelLogPipelineHealthy(t, pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t, backend, genNs)

			// Determine instance ID suffix and version
			serviceVersion := telemetrygen.GetVersion()
			serviceInstanceIDSuffix := "telemetrygen"

			if suite.ExpectAgent(tc.label) {
				serviceVersion = stdoutloggen.GetVersion()
				serviceInstanceIDSuffix = stdoutloggen.DefaultContainerName
			}

			// Logs from pod with no annotations should be enriched (agent only)
			if suite.ExpectAgent(tc.label) {
				verifyServiceAttributes(t, backend, podWithNoAnnotationsName, ServiceAttributes{
					ServiceName:       podWithNoAnnotationsName,
					ServiceNamespace:  genNs,
					ServiceVersion:    stdoutloggen.GetVersion(),
					ServiceInstanceID: fmt.Sprintf("%s.%s.%s", genNs, podWithNoAnnotationsName, stdoutloggen.DefaultContainerName),
				})
			}

			// Empty attributes should be enriched
			verifyServiceAttributes(t, backend, podWithEmptyServiceAttributesName, ServiceAttributes{
				ServiceName:       podWithEmptyServiceAttributesName,
				ServiceNamespace:  genNs,
				ServiceVersion:    serviceVersion,
				ServiceInstanceID: fmt.Sprintf("%s.%s.%s", genNs, podWithEmptyServiceAttributesName, serviceInstanceIDSuffix),
			})

			// Unknown service names should be enriched (gateway only)
			if !suite.ExpectAgent(tc.label) {
				verifyServiceAttributes(t, backend, podWithUnknownServiceName, ServiceAttributes{
					ServiceName: podWithUnknownServiceName,
				})
				verifyServiceAttributes(t, backend, podWithUnknownServicePatternName, ServiceAttributes{
					ServiceName: podWithUnknownServicePatternName,
				})
			}

			// Custom attributes should be preserved
			verifyServiceAttributes(t, backend, podWithCustomServiceAttributesName, ServiceAttributes{
				ServiceName:       customServiceName,
				ServiceNamespace:  customServiceNamespace,
				ServiceVersion:    customServiceVersion,
				ServiceInstanceID: customServiceInstanceID,
			})

			// Verify that temporary kyma resource attributes are removed from the logs
			assert.BackendDataConsistentlyMatches(t, backend,
				HaveFlatLogs(Not(ContainElement(
					HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))),
				))),
			)
		})
	}
}

type ServiceAttributes struct {
	ServiceName       string
	ServiceNamespace  string
	ServiceVersion    string
	ServiceInstanceID string
}

func verifyServiceAttributes(t *testing.T, backend *kitbackend.Backend, givenPodPrefix string, expectedAttributes ServiceAttributes) {
	t.Helper()

	var matchers []gomegatypes.GomegaMatcher

	matchers = append(matchers, HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(givenPodPrefix))))

	if expectedAttributes.ServiceName != "" {
		matchers = append(matchers, HaveResourceAttributes(HaveKeyWithValue("service.name", expectedAttributes.ServiceName)))
	}

	if expectedAttributes.ServiceNamespace != "" {
		matchers = append(matchers, HaveResourceAttributes(HaveKeyWithValue("service.namespace", expectedAttributes.ServiceNamespace)))
	}

	if expectedAttributes.ServiceVersion != "" {
		matchers = append(matchers, HaveResourceAttributes(HaveKeyWithValue("service.version", expectedAttributes.ServiceVersion)))
	}

	if expectedAttributes.ServiceInstanceID != "" {
		matchers = append(matchers, HaveResourceAttributes(HaveKeyWithValue("service.instance.id", expectedAttributes.ServiceInstanceID)))
	}

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatLogs(ContainElement(SatisfyAll(matchers...))),
	)
}
