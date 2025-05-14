package shared

import (
	"context"
	"testing"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDisabledInput_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogAgent)

	const (
		endpoint = "localhost:443"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		mockNs       = uniquePrefix()
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
		kitk8s.NewNamespace(mockNs).K8sObject(),
		&pipeline,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	Consistently(func(g Gomega) {
		var daemonSet appsv1.DaemonSet
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.LogAgentName, &daemonSet)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
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
		mockNs       = uniquePrefix()
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
		kitk8s.NewNamespace(mockNs).K8sObject(),
		&pipeline,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeAgentHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonAgentNotReady,
	})

	Consistently(func(g Gomega) {
		var daemonSet appsv1.DaemonSet
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitDaemonSetName, &daemonSet)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
}
