package logagent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

const (
	systemNamespacesIncluded = true
	systemNamespacesExcluded = false
)

func TestReceiverCreator(t *testing.T) {
	expectedExcludePaths := getExcludePaths(systemNamespacesIncluded)
	expectedIncludePaths := []string{"/var/log/pods/*_*/*/*.log"}

	tt := []struct {
		name              string
		pipeline          telemetryv1beta1.LogPipeline
		expectedOperators []Operator
	}{
		{
			name:     "should create receiver with keepOriginalBody true",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithKeepOriginalBody(true).Build(),
			expectedOperators: []Operator{
				makeContainerParser(),
				makeMoveToLogStream(),
				makeDropAttributeLogTag(),
				makeBodyRouter(),
				makeJSONParser(),
				makeMoveBodyToLogOriginal(),
				makeMoveMessageToBody(),
				makeMoveMsgToBody(),
				makeSeverityParserFromLevel(),
				makeRemoveLevel(),
				makeSeverityParserFromLogLevel(),
				makeRemoveLogLevel(),
				makeTraceRouter(),
				makeTraceParentParser(),
				makeTraceParser(),
				makeRemoveTraceParent(),
				makeRemoveTraceID(),
				makeRemoveSpanID(),
				makeRemoveTraceFlags(),
				makeNoop(),
			},
		},
		{
			name:     "should create receiver with keepOriginalBody false",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithKeepOriginalBody(false).Build(),
			expectedOperators: []Operator{
				makeContainerParser(),
				makeMoveToLogStream(),
				makeDropAttributeLogTag(),
				makeBodyRouter(),
				makeJSONParser(),
				makeRemoveBody(),
				makeMoveMessageToBody(),
				makeMoveMsgToBody(),
				makeSeverityParserFromLevel(),
				makeRemoveLevel(),
				makeSeverityParserFromLogLevel(),
				makeRemoveLogLevel(),
				makeTraceRouter(),
				makeTraceParentParser(),
				makeTraceParser(),
				makeRemoveTraceParent(),
				makeRemoveTraceID(),
				makeRemoveSpanID(),
				makeRemoveTraceFlags(),
				makeNoop(),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fileLogReceiver := fileLogReceiverConfig(&tc.pipeline, false)
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
	expectedContainerParser := Operator{
		ID:                      "containerd-parser",
		Type:                    "container",
		AddMetadataFromFilePath: ptr.To(true),
		Format:                  "containerd",
	}
	assert.Equal(t, expectedContainerParser, cp)
}

func TestMakeMoveToLogStream(t *testing.T) {
	mtls := makeMoveToLogStream()
	expectedMoveToLogStream := Operator{
		ID:     "move-to-log-stream",
		Type:   "move",
		From:   "attributes[\"stream\"]",
		To:     "attributes[\"log.iostream\"]",
		IfExpr: "attributes[\"stream\"] != nil",
	}
	assert.Equal(t, expectedMoveToLogStream, mtls)
}

func TestExpectedMakeBodyRouter(t *testing.T) {
	jp := makeBodyRouter()
	expectedJP := Operator{
		ID:      "body-router",
		Type:    "router",
		Default: "noop",
		Routes: []Route{
			{
				Expression: "body matches '^{.*}$'",
				Output:     "json-parser",
			},
		},
	}
	assert.Equal(t, expectedJP, jp)
}

func TestExpectedMakeJSONParser(t *testing.T) {
	jp := makeJSONParser()
	expectedJP := Operator{
		ID:        "json-parser",
		Type:      "json_parser",
		ParseFrom: "body",
		ParseTo:   "attributes",
	}
	assert.Equal(t, expectedJP, jp)
}

func TestMakeMoveBodyToLogOriginal(t *testing.T) {
	mbto := makeMoveBodyToLogOriginal()
	expectedCBTO := Operator{
		ID:   "move-body-to-attributes-log-original",
		Type: "move",
		From: "body",
		To:   "attributes[\"log.original\"]",
	}
	assert.Equal(t, expectedCBTO, mbto)
}

func TestMakeRemoveBody(t *testing.T) {
	mbto := makeRemoveBody()
	expectedCBTO := Operator{
		ID:    "remove-body",
		Type:  "remove",
		Field: "body",
	}
	assert.Equal(t, expectedCBTO, mbto)
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
		From:   "attributes[\"message\"]",
		To:     "body",
		IfExpr: "attributes[\"message\"] != nil",
	}
	assert.Equal(t, expectedMMTB, mmtb)
}

func TestMakeMoveMsgToBody(t *testing.T) {
	mmtb := makeMoveMsgToBody()
	expectedMMTB := Operator{
		ID:     "move-msg-to-body",
		Type:   "move",
		From:   "attributes[\"msg\"]",
		To:     "body",
		IfExpr: "attributes[\"msg\"] != nil",
	}
	assert.Equal(t, expectedMMTB, mmtb)
}

func TestMakeSeverityParser(t *testing.T) {
	sp := makeSeverityParserFromLevel()
	expectedSP := Operator{
		ID:        "parse-level",
		Type:      "severity_parser",
		ParseFrom: "attributes[\"level\"]",
		IfExpr:    "attributes[\"level\"] != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestExcludePath(t *testing.T) {
	tt := []struct {
		name     string
		pipeline telemetryv1beta1.LogPipeline
		expected []string
	}{
		{
			name:     "should return excluded path if namespace is present",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithExcludeNamespaces("foo", "bar").Build(),
			expected: getExcludePaths(systemNamespacesExcluded, "/var/log/pods/foo_*/*/*.log", "/var/log/pods/bar_*/*/*.log"),
		},
		{
			name:     "should return default excluded path if namespace is not present",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithKeepOriginalBody(false).Build(),
			expected: getExcludePaths(systemNamespacesIncluded),
		},
		{
			name:     "Should include excluded container if present",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithExcludeContainers("foo", "bar").Build(),
			expected: getExcludePaths(systemNamespacesIncluded, "/var/log/pods/*_*/foo/*.log", "/var/log/pods/*_*/bar/*.log"),
		},
		{
			name:     "Should include system namespaces when empty namespace selector is provided",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithExcludeNamespaces().Build(),
			expected: getExcludePaths(systemNamespacesExcluded),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			excludePaths := createExcludePath(tc.pipeline.Spec.Input.Runtime, false)
			require.Equal(t, tc.expected, excludePaths)
		})
	}
}

func TestIncludePath(t *testing.T) {
	tt := []struct {
		name     string
		pipeline telemetryv1beta1.LogPipeline
		expected []string
	}{
		{
			name:     "should return included path if namespace is present",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithKeepOriginalBody(true).WithIncludeNamespaces("foo", "bar").Build(),
			expected: []string{
				"/var/log/pods/foo_*/*/*.log",
				"/var/log/pods/bar_*/*/*.log",
			},
		},
		{
			name:     "should return default included path if namespace is not present",
			pipeline: testutils.NewLogPipelineBuilder().WithRuntimeInput(true).WithKeepOriginalBody(false).Build(),
			expected: []string{"/var/log/pods/*_*/*/*.log"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			includePaths := createIncludePath(tc.pipeline.Spec.Input.Runtime)
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
			ParseFrom: "attributes[\"trace_id\"]",
		},
		SpanID: OperatorAttribute{
			ParseFrom: "attributes[\"span_id\"]",
		},
		TraceFlags: OperatorAttribute{
			ParseFrom: "attributes[\"trace_flags\"]",
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
		ParseFrom: "attributes[\"traceparent\"]",
		Output:    "remove-trace-parent",
		Trace: TraceAttribute{
			TraceID: OperatorAttribute{
				ParseFrom: "attributes[\"trace_id\"]",
			},
			SpanID: OperatorAttribute{
				ParseFrom: "attributes[\"span_id\"]",
			},
			TraceFlags: OperatorAttribute{
				ParseFrom: "attributes[\"trace_flags\"]",
			},
		},
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeRemoveTraceParent(t *testing.T) {
	sp := makeRemoveTraceParent()
	expectedSP := Operator{
		ID:    "remove-trace-parent",
		Type:  "remove",
		Field: "attributes[\"traceparent\"]",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeRemoveTraceID(t *testing.T) {
	sp := makeRemoveTraceID()
	expectedSP := Operator{
		ID:     "remove-trace-id",
		Type:   "remove",
		Field:  "attributes[\"trace_id\"]",
		IfExpr: "attributes[\"trace_id\"] != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeRemoveSpanID(t *testing.T) {
	sp := makeRemoveSpanID()
	expectedSP := Operator{
		ID:     "remove-span-id",
		Type:   "remove",
		Field:  "attributes[\"span_id\"]",
		IfExpr: "attributes[\"span_id\"] != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeRemoveTraceFlags(t *testing.T) {
	sp := makeRemoveTraceFlags()
	expectedSP := Operator{
		ID:     "remove-trace-flags",
		Type:   "remove",
		Field:  "attributes[\"trace_flags\"]",
		IfExpr: "attributes[\"trace_flags\"] != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeNoop(t *testing.T) {
	sp := makeNoop()
	expectedSP := Operator{
		ID:   "noop",
		Type: "noop",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeTraceRouter(t *testing.T) {
	sp := makeTraceRouter()
	expectedSP := Operator{
		ID:      "trace-router",
		Type:    "router",
		Default: "noop",
		Routes: []Route{
			{
				Expression: "attributes[\"trace_id\"] != nil",
				Output:     "trace-parser",
			},
			{
				Expression: fmt.Sprintf("attributes[\"traceparent\"] != nil and attributes[\"traceparent\"] matches '%s'", traceParentExpression),
				Output:     "trace-parent-parser",
			},
		},
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeSeverityParserFromLogLevel(t *testing.T) {
	sp := makeSeverityParserFromLogLevel()
	expectedSP := Operator{
		ID:        "parse-log-level",
		Type:      "severity_parser",
		IfExpr:    "attributes[\"log.level\"] != nil",
		ParseFrom: "attributes[\"log.level\"]",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeSeverityParserFromLevel(t *testing.T) {
	sp := makeSeverityParserFromLevel()
	expectedSP := Operator{
		ID:        "parse-level",
		Type:      "severity_parser",
		ParseFrom: "attributes[\"level\"]",
		IfExpr:    "attributes[\"level\"] != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeRemoveLogLevel(t *testing.T) {
	sp := makeRemoveLogLevel()
	expectedSP := Operator{
		ID:     "remove-log-level",
		Type:   "remove",
		Field:  "attributes[\"log.level\"]",
		IfExpr: "attributes[\"log.level\"] != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func TestMakeRemoveLevel(t *testing.T) {
	sp := makeRemoveLevel()
	expectedSP := Operator{
		ID:     "remove-level",
		Type:   "remove",
		Field:  "attributes[\"level\"]",
		IfExpr: "attributes[\"level\"] != nil",
	}
	assert.Equal(t, expectedSP, sp)
}

func getExcludePaths(system bool, paths ...string) []string {
	var defaultExcludePaths = []string{
		"/var/log/pods/kyma-system_telemetry-fluent-bit-*/fluent-bit/*.log",
		"/var/log/pods/kyma-system_telemetry-log-agent-*/collector/*.log",
		"/var/log/pods/kyma-system_*system-logs-agent-*/collector/*.log",
		"/var/log/pods/kyma-system_*system-logs-collector-*/collector/*.log",
	}

	var systemExcludePaths = []string{
		"/var/log/pods/kyma-system_*/*/*.log",
		"/var/log/pods/kube-system_*/*/*.log",
		"/var/log/pods/istio-system_*/*/*.log",
	}

	excludePaths := []string{}
	excludePaths = append(excludePaths, defaultExcludePaths...)

	if system {
		excludePaths = append(excludePaths, systemExcludePaths...)
	}

	if len(paths) == 0 {
		return excludePaths
	}

	return append(excludePaths, paths...)
}
