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
		"/var/log/pods/kyma-system_telemetry-log-agent*/*/*.log",
		"/var/log/pods/kyma-system_telemetry-fluent-bit*/*/*.log",
	}
	expectedIncludePaths := []string{"/var/log/pods/*/*/*.log"}
	tt := []struct {
		name              string
		pipelines         []telemetryv1alpha1.LogPipeline
		expectedOperators []Operator
	}{
		{
			name:      "should create receiver with keepOriginalBody true",
			pipelines: []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()},
			expectedOperators: []Operator{
				makeContainerParser(),
				makeMoveToLogStream(),
				makeJSONParser(),
				makeCopyBodyToOriginal(),
				makeMoveMessageToBody(),
				makeMoveMsgToBody(),
				makeSeverityParser(),
			},
		},
		{
			name:      "should create receiver with keepOriginalBody false",
			pipelines: []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(false).Build()},
			expectedOperators: []Operator{
				makeContainerParser(),
				makeMoveToLogStream(),
				makeJSONParser(),
				makeMoveMessageToBody(),
				makeMoveMsgToBody(),
				makeSeverityParser(),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			receivers := makeReceivers(tc.pipelines, BuildOptions{AgentNamespace: "kyma-system"})
			require.Equal(t, expectedExcludePaths, receivers.FileLog.Exclude)
			require.Equal(t, expectedIncludePaths, receivers.FileLog.Include)
			require.Equal(t, false, receivers.FileLog.IncludeFileName)
			require.Equal(t, true, receivers.FileLog.IncludeFilePath)
			require.Equal(t, tc.expectedOperators, receivers.FileLog.Operators)
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
