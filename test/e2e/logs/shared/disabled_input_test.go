package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestDisabledInput_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(false).
		WithOTLPInput(false).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.EndpointHTTP()),
		).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeLogs).K8sObject(),
	}

	resources = append(resources, backend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.LogGatewayName)
	assert.OTelLogPipelineHealthy(t, pipelineName)

	// If Runtime input is disabled, THEN the log agent must not be deployed
	Eventually(func(g Gomega) {
		var daemonSet appsv1.DaemonSet

		err := suite.K8sClient.Get(t.Context(), kitkyma.LogAgentName, &daemonSet)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Log agent DaemonSet must not exist")
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).To(Succeed())

	// If OTLP input is disabled, THEN the logs pushed to the gateway should not be sent to the backend
	assert.BackendDataConsistentlyMatches(t, backend, HaveFlatLogs(BeEmpty()))
}

func TestDisabledInput_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	const (
		endpointAddress = "localhost"
		endpointPort    = 443
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(false).
		WithHTTPOutput(
			testutils.HTTPHost(endpointAddress),
			testutils.HTTPPort(endpointPort),
		).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	Eventually(func(g Gomega) {
		var daemonSet appsv1.DaemonSet

		err := suite.K8sClient.Get(t.Context(), kitkyma.FluentBitDaemonSetName, &daemonSet)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Fluent Bit DaemonSet must not exist")
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).To(Succeed())
}
