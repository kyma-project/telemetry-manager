package traces

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestServiceEnrichment(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	const (
		// pod names
		podWithEmptyServiceAttributesName  = "pod-with-empty-service"
		podWithUnknownServiceName          = "pod-with-unknown-service"
		podWithUnknownServicePatternName   = "pod-with-unknown-service-pattern"
		podWithCustomServiceAttributesName = "pod-with-custom-service"

		// misc
		unknownService          = "unknown_service"
		unknownServicePattern   = "unknown_service:bash"
		customServiceName       = "custom-service"
		customServiceNamespace  = "custom-namespace"
		customServiceVersion    = "v1.2.3"
		customServiceInstanceID = "instance-1234"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")

		telemetry operatorv1beta1.Telemetry
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	// Configure generator pods
	podSpecWithEmptyServiceAttributes := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
		telemetrygen.WithServiceName(""),
		telemetrygen.WithServiceNamespace(""),
		telemetrygen.WithServiceVersion(""),
		telemetrygen.WithServiceInstanceID(""),
	)
	podSpecWithUnknownServiceName := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
		telemetrygen.WithServiceName(unknownService))
	podSpecWithUnknownServiceNamePattern := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
		telemetrygen.WithServiceName(unknownServicePattern))
	podSpecWithCustomServiceAttributes := telemetrygen.PodSpec(telemetrygen.SignalTypeTraces,
		telemetrygen.WithServiceName(customServiceName),
		telemetrygen.WithServiceNamespace(customServiceNamespace),
		telemetrygen.WithServiceVersion(customServiceVersion),
		telemetrygen.WithServiceInstanceID(customServiceInstanceID),
	)

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
		kitk8sobjects.NewPod(podWithEmptyServiceAttributesName, genNs).WithPodSpec(podSpecWithEmptyServiceAttributes).K8sObject(),
		kitk8sobjects.NewPod(podWithUnknownServiceName, genNs).WithPodSpec(podSpecWithUnknownServiceName).K8sObject(),
		kitk8sobjects.NewPod(podWithUnknownServicePatternName, genNs).WithPodSpec(podSpecWithUnknownServiceNamePattern).K8sObject(),
		kitk8sobjects.NewPod(podWithCustomServiceAttributesName, genNs).WithPodSpec(podSpecWithCustomServiceAttributes).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	// Empty attributes should be enriched
	verifyServiceAttributes(t, backend, podWithEmptyServiceAttributesName, ServiceAttributes{
		ServiceName:       podWithEmptyServiceAttributesName,
		ServiceNamespace:  genNs,
		ServiceVersion:    telemetrygen.GetVersion(),
		ServiceInstanceID: fmt.Sprintf("%s.%s.telemetrygen", genNs, podWithEmptyServiceAttributesName),
	})

	// Unknown service names should be enriched
	verifyServiceAttributes(t, backend, podWithUnknownServiceName, ServiceAttributes{
		ServiceName: podWithUnknownServiceName,
	})
	verifyServiceAttributes(t, backend, podWithUnknownServicePatternName, ServiceAttributes{
		ServiceName: podWithUnknownServicePatternName,
	})

	// Custom attributes should be preserved
	verifyServiceAttributes(t, backend, podWithCustomServiceAttributesName, ServiceAttributes{
		ServiceName:       customServiceName,
		ServiceNamespace:  customServiceNamespace,
		ServiceVersion:    customServiceVersion,
		ServiceInstanceID: customServiceInstanceID,
	})

	// Verify that temporary kyma resource attributes are removed from the spans
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatTraces(
			Not(ContainElement(HaveResourceAttributes(HaveKey(ContainSubstring("kyma"))))),
		), assert.WithOptionalDescription("Should have no kyma resource attributes"),
	)
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
		HaveFlatTraces(ContainElement(SatisfyAll(matchers...))),
	)
}
