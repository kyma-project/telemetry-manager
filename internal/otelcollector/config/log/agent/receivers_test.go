package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestReceiverCreator(t *testing.T) {
	expectedExcludePaths := []string{
		"/var/log/pods/kyma-system_*/*/*.log",
		"/var/log/pods/kube-system_*/*/*.log",
		"/var/log/pods/istio-system_*/*/*.log",
		"/var/log/pods/compass-system_*/*/*.log",
		"/var/log/pods/kyma-system_telemetry-log-agent*/*/*.log",
		"/var/log/pods/kyma-system_telemetry-fluent-bit*/*/*.log",
		"/var/log/pods/kyma-system_*system-logs-collector*/*/*.log",
		"/var/log/pods/kyma-system_*system-logs-agent*/*/*.log",
	}
	expectedIncludePaths := []string{"/var/log/pods/*/*/*.log"}

	tt := []struct {
		name              string
		pipeline          telemetryv1alpha1.LogPipeline
		expectedOperators []Operator
	}{
		{
			name:     "should create receiver with keepOriginalBody true",
			pipeline: testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build(),
			expectedOperators: append(append([]Operator{
				makeContainerParser(),
				makeMoveToLogStream(),
				makeDropAttributeLogTag(),
				makeJSONParser(),
				makeCopyBodyToOriginal(),
				makeMoveMessageToBody(),
				makeMoveMsgToBody(),
				makeSeverityParser(),
				makeTraceRouter(),
				makeTraceParentParser(),
				makeTraceParser(),
			}, makeRemoveTraceAttributes()...),
				makeNoop()),
		},
		{
			name:     "should create receiver with keepOriginalBody false",
			pipeline: testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(false).Build(),
			expectedOperators: append(append([]Operator{
				makeContainerParser(),
				makeMoveToLogStream(),
				makeDropAttributeLogTag(),
				makeJSONParser(),
				makeMoveMessageToBody(),
				makeMoveMsgToBody(),
				makeSeverityParser(),
				makeTraceRouter(),
				makeTraceParentParser(),
				makeTraceParser(),
			}, makeRemoveTraceAttributes()...),
				makeNoop()),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fileLogReceiver := makeFileLogReceiver(tc.pipeline, BuildOptions{AgentNamespace: "kyma-system"})
			require.Equal(t, expectedExcludePaths, fileLogReceiver.Exclude)
			require.Equal(t, expectedIncludePaths, fileLogReceiver.Include)
			require.Equal(t, ptr.To(false), fileLogReceiver.IncludeFileName)
			require.Equal(t, ptr.To(true), fileLogReceiver.IncludeFilePath)
			require.Equal(t, tc.expectedOperators, fileLogReceiver.Operators)
		})
	}
}

func TestMakeContainerParser(t *testing.T) {
	cp := makeContainerParser()
	expectedConainerParser := Operator{
		ID:                      "containerd-parser",
		Type:                    "container",
		AddMetadataFromFilePath: ptr.To(true),
		Format:                  "containerd",
	}
	assert.Equal(t, expectedConainerParser, cp)
}

func TestMakeMoveToLogStream(t *testing.T) {
	mtls := makeMoveToLogStream()
	expectedMoveToLogStream := Operator{
		ID:     "move-to-log-stream",
		Type:   "move",
		From:   "attributes.stream",
		To:     "attributes[\"log.iostream\"]",
		IfExpr: "attributes.stream != nil",
	}
	assert.Equal(t, expectedMoveToLogStream, mtls)
}

func TestExpectedMakeJSONParser(t *testing.T) {
	jp := makeJSONParser()
	expectedJP := Operator{
		ID:        "json-parser",
		Type:      "json_parser",
		ParseFrom: "body",
		ParseTo:   "attributes",
		IfExpr:    "body matches '^{.*}$'",
	}
	assert.Equal(t, expectedJP, jp)
}

func TestMakeCopyBodyToOriginal(t *testing.T) {
	cbto := makeCopyBodyToOriginal()
	expectedCBTO := Operator{
		ID:   "copy-body-to-attributes-original",
		Type: "copy",
		From: "body",
		To:   "attributes.original",
	}
	assert.Equal(t, expectedCBTO, cbto)
}

func TestMakeDropAttributeLogTag(t *testing.T) {
	dalt := makeDropAttributeLogTag()
	expectedDALT := Operator{
		ID:    "drop-attribute-log-tag",
		Type:  "remove",
		Field: "attributes[\"logtag\"]",
	}
	assert.Equal(t, expectedDALT, dalt)
}

func TestMakeMoveMessageToBody(t *testing.T) {
	mmtb := makeMoveMessageToBody()
	expectedMMTB := Operator{

		ID:     "move-message-to-body",
		Type:   "move",
		From:   "attributes.message",
		To:     "body",
		IfExpr: "attributes.message != nil",
	}
	assert.Equal(t, expectedMMTB, mmtb)
}

func TestMakeMoveMsgToBody(t *testing.T) {
	mmtb := makeMoveMsgToBody()
	expectedMMTB := Operator{
		ID:     "move-msg-to-body",
		Type:   "move",
		From:   "attributes.msg",
		To:     "body",
		IfExpr: "attributes.msg != nil",
	}
	assert.Equal(t, expectedMMTB, mmtb)
}

func TestMakeSeverityParser(t *testing.T) {
	sp := makeSeverityParser()
	expectedSP := Operator{
		ID:        "severity-parser",
		Type:      "severity_parser",
		ParseFrom: "attributes.level",
		IfExpr:    "attributes.level != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestExcludePath(t *testing.T) {
	tt := []struct {
		name     string
		pipeline telemetryv1alpha1.LogPipeline
		expected []string
	}{
		{
			name:     "should return excluded path if namespace is present",
			pipeline: testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).WithExcludeNamespaces("foo", "bar").Build(),
			expected: []string{
				"/var/log/pods/kyma-system_*/*/*.log",
				"/var/log/pods/kube-system_*/*/*.log",
				"/var/log/pods/istio-system_*/*/*.log",
				"/var/log/pods/compass-system_*/*/*.log",
				"/var/log/pods/foo_*/*/*.log",
				"/var/log/pods/bar_*/*/*.log",
			},
		},
		{
			name:     "should return empty excluded path if namespace is not present",
			pipeline: testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(false).Build(),
			expected: []string{
				"/var/log/pods/kyma-system_*/*/*.log",
				"/var/log/pods/kube-system_*/*/*.log",
				"/var/log/pods/istio-system_*/*/*.log",
				"/var/log/pods/compass-system_*/*/*.log",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			excludePaths := createExcludePath(tc.pipeline.Spec.Input.Application)
			require.Equal(t, tc.expected, excludePaths)
		})
	}
}

func TestIncludePath(t *testing.T) {
	tt := []struct {
		name     string
		pipeline telemetryv1alpha1.LogPipeline
		expected []string
	}{
		{
			name:     "should return included path if namespace is present",
			pipeline: testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).WithIncludeNamespaces("foo", "bar").Build(),
			expected: []string{
				"/var/log/pods/foo_*/*/*.log",
				"/var/log/pods/bar_*/*/*.log",
			},
		},
		{
			name:     "should return default included path if namespace is not present",
			pipeline: testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(false).Build(),
			expected: []string{"/var/log/pods/*/*/*.log"},
		},
		{
			name:     "should return system namespaces included path if system is true",
			pipeline: testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(false).WithSystemNamespaces(true).Build(),
			expected: []string{
				"/var/log/pods/kyma-system_*/*/*.log",
				"/var/log/pods/kube-system_*/*/*.log",
				"/var/log/pods/istio-system_*/*/*.log",
				"/var/log/pods/compass-system_*/*/*.log",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			includePaths := createIncludePath(tc.pipeline.Spec.Input.Application)
			require.Equal(t, tc.expected, includePaths)
		})
	}
}

func TestMakeTraceParser(t *testing.T) {
	sp := makeTraceParser()
	expectedSP := Operator{
		ID:     "trace-parser",
		Type:   "trace_parser",
		Output: "remove-trace-id",
		TraceID: OperatorAttribute{
			ParseFrom: "attributes.trace_id",
		},
		SpanID: OperatorAttribute{
			ParseFrom: "attributes.span_id",
		},
		TraceFlags: OperatorAttribute{
			ParseFrom: "attributes.trace_flags",
		},
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeTraceParentParser(t *testing.T) {
	sp := makeTraceParentParser()
	expectedSP := Operator{
		ID:        "trace-parent-parser",
		Type:      "regex_parser",
		Regex:     traceParentExpression,
		ParseFrom: "attributes.traceparent",
		Output:    "remove-trace-parent",
		Trace: TraceAttribute{
			TraceID: OperatorAttribute{
				ParseFrom: "attributes.trace_id",
			},
			SpanID: OperatorAttribute{
				ParseFrom: "attributes.span_id",
			},
			TraceFlags: OperatorAttribute{
				ParseFrom: "attributes.trace_flags",
			},
		},
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeRemoveTraceAttributes(t *testing.T) {
	sp := makeRemoveTraceAttributes()
	expectedSP := []Operator{
		{
			ID:     "remove-trace-id",
			Type:   "remove",
			Field:  "attributes.trace_id",
			Output: "remove-span-id",
		},
		{
			ID:     "remove-span-id",
			Type:   "remove",
			Field:  "attributes.span_id",
			Output: "remove-trace-flags",
		},
		{
			ID:    "remove-trace-flags",
			Type:  "remove",
			Field: "attributes.trace_flags",
		},
		{
			ID:     "remove-traceparent",
			Type:   "remove",
			Field:  "attributes.traceparent",
			Output: "remove-trace-id",
		},
	}
	assert.Equal(t, expectedSP, sp)
}
