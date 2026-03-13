package metricpipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestOTLPOutputPort(t *testing.T) {
	fakeClient := fake.NewFakeClient()

	t.Run("metric pipelines all have valid endpoints", func(t *testing.T) {
		metricPipelines := []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://sample.test.com:4317")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com:4318/api/test")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("http://sample.test.com")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com:9090/api/test")).Build(),
		}

		ports, err := OTLPOutputPorts(t.Context(), fakeClient, metricPipelines)
		require.NoError(t, err)
		require.Equal(t, []string{"4317", "4318", "9090"}, ports)
	})

	t.Run("some metric pipelines have invalid endpoints", func(t *testing.T) {
		metricPipelines := []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://sample.test.com:4317")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("://sample.test.com/:4318/api/test")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com:9090/api/test")).Build(),
		}

		ports, err := OTLPOutputPorts(t.Context(), fakeClient, metricPipelines)
		require.NoError(t, err)
		require.Equal(t, []string{"4317", "9090"}, ports)
	})

	t.Run("all metric pipelines have invalid endpoints", func(t *testing.T) {
		metricPipelines := []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com/api/test")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("grpc://sample.test.com")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint(":9090.sample.test.com/api/test")).Build(),
		}

		ports, err := OTLPOutputPorts(t.Context(), fakeClient, metricPipelines)
		require.NoError(t, err)
		require.Equal(t, []string{}, ports)
	})

	t.Run("duplicated ports get compacted", func(t *testing.T) {
		metricPipelines := []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://sample.test.com")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("http://sample.test.com/api/test")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("http://sample.test.com:80")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com:9090/api/test")).Build(),
		}

		ports, err := OTLPOutputPorts(t.Context(), fakeClient, metricPipelines)
		require.NoError(t, err)
		require.Equal(t, []string{"80", "9090"}, ports)
	})
	t.Run("ports get sorted properly", func(t *testing.T) {
		metricPipelines := []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("http://sample.test.com:7070/api/test")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("http://sample.test.com:80")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("sample.test.com:9090/api/test")).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("https://sample.test.com:4317")).Build(),
		}

		ports, err := OTLPOutputPorts(t.Context(), fakeClient, metricPipelines)
		require.NoError(t, err)
		require.Equal(t, []string{"4317", "7070", "80", "9090"}, ports)
	})

	t.Run("otlp/http output with no port gets the default port", func(t *testing.T) {
		metricPipelines := []telemetryv1beta1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithOTLPOutput(testutils.OTLPEndpoint("http://sample.test.com/api/test"), testutils.OTLPProtocol(telemetryv1beta1.OTLPProtocolHTTP)).Build(),
		}

		ports, err := OTLPOutputPorts(t.Context(), fakeClient, metricPipelines)
		require.NoError(t, err)
		require.Equal(t, []string{"4318"}, ports)
	})
}
