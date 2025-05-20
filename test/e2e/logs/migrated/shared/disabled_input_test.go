package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestDisabledInput_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	const (
		endpoint = "localhost:443"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithApplicationInput(false).
		WithOTLPInput(false).
		WithOTLPOutput(
			testutils.OTLPEndpoint(endpoint),
		).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	Eventually(func(g Gomega) {
		var daemonSet appsv1.DaemonSet
		err := suite.K8sClient.Get(t.Context(), kitkyma.LogAgentName, &daemonSet)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Log agent DaemonSet must not exist")
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
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
		WithApplicationInput(false).
		WithHTTPOutput(
			testutils.HTTPHost(endpointAddress),
			testutils.HTTPPort(endpointPort),
		).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	Eventually(func(g Gomega) {
		var daemonSet appsv1.DaemonSet
		err := suite.K8sClient.Get(t.Context(), kitkyma.FluentBitDaemonSetName, &daemonSet)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Fluent Bit DaemonSet must not exist")
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
