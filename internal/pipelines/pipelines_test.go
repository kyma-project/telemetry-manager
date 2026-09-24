package pipelines

import (
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestSignalTypeValidate(t *testing.T) {
	for _, valid := range []SignalType{SignalTypeTrace, SignalTypeMetric, SignalTypeLog, SignalTypeLogFluentBit} {
		require.NoError(t, valid.Validate())
	}

	require.Error(t, SignalType("unknown").Validate())
}

func TestLogPipelineRef(t *testing.T) {
	lp := &telemetryv1beta1.LogPipeline{Name: "my-log"}
	ref := LogPipelineRef(lp)
	require.Equal(t, "my-log", ref.Name())
	require.Equal(t, SignalTypeLog, ref.SignalType())
	require.Equal(t, "logpipeline", ref.TypePrefix())
	require.Equal(t, "logpipeline-my-log", ref.QualifiedName())
}

func TestMetricPipelineRef(t *testing.T) {
	mp := &telemetryv1beta1.MetricPipeline{Name: "my-metric"}
	ref := MetricPipelineRef(mp)
	require.Equal(t, "my-metric", ref.Name())
	require.Equal(t, SignalTypeMetric, ref.SignalType())
	require.Equal(t, "metricpipeline", ref.TypePrefix())
	require.Equal(t, "metricpipeline-my-metric", ref.QualifiedName())
}

func TestTracePipelineRef(t *testing.T) {
	tp := &telemetryv1beta1.TracePipeline{Name: "my-trace"}
	ref := TracePipelineRef(tp)
	require.Equal(t, "my-trace", ref.Name())
	require.Equal(t, SignalTypeTrace, ref.SignalType())
	require.Equal(t, "tracepipeline", ref.TypePrefix())
	require.Equal(t, "tracepipeline-my-trace", ref.QualifiedName())
}

func TestPipelineRefEmptySignalType(t *testing.T) {
	ref := PipelineRef{name: "orphan"}
	require.Equal(t, "", ref.TypePrefix())
	require.Equal(t, "orphan", ref.QualifiedName())
}
