package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineBroken_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix   = unique.Prefix()
		backendNs      = uniquePrefix("backend")
		generatorNs    = uniquePrefix("gen")
		goodPipeline   = uniquePrefix("good")
		brokenPipeline = uniquePrefix("broken")
	)

	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit)
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	logPipelineGood := testutils.NewLogPipelineBuilder().
		WithName(goodPipeline).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	logPipelineBroken := testutils.NewLogPipelineBuilder().
		WithName(brokenPipeline).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHostFromSecret("dummy", "dummy", "dummy")). // broken pipeline references a secret that does not exist
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		&logPipelineGood,
		&logPipelineBroken,
		loggen.New(generatorNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, logPipelineGood.Name)
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, logPipelineBroken.Name, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)
}
