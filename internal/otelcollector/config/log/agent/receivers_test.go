package agent

import (
	"fmt"
	"testing"

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
			name:              "should create receiver with keepOriginalBody true",
			pipelines:         []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()},
			expectedOperators: makeExpectedOperators(),
		},
		{
			name:              "should create receiver with keepOriginalBody false",
			pipelines:         []telemetryv1alpha1.LogPipeline{testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(false).Build()},
			expectedOperators: makeExpectedOperatorsWithoutKeepOringinalBody(),
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

func makeExpectedOperators() []Operator {
	return []Operator{
		expectedMakeContainerParser(),
		expectedMakeMoveToLogStream(),
		expectedMakeJSONParser(),
		expectedMakeCopyBodyToOriginal(),
		expectedMakeMoveMessageToBody(),
		expectedMakeMoveMsgToBody(),
		expectedMakeSeverityParser(),
	}
}

func makeExpectedOperatorsWithoutKeepOringinalBody() []Operator {
	return []Operator{
		expectedMakeContainerParser(),
		expectedMakeMoveToLogStream(),
		expectedMakeJSONParser(),
		expectedMakeMoveMessageToBody(),
		expectedMakeMoveMsgToBody(),
		expectedMakeSeverityParser(),
	}
}

// parse the log with containerd parser
func expectedMakeContainerParser() Operator {
	return Operator{
		ID:                      "containerd-parser",
		Type:                    "container",
		AddMetadataFromFilePath: ptr.To(true),
		Format:                  "containerd",
	}
}

// move the stream to log.iostream
func expectedMakeMoveToLogStream() Operator {
	return Operator{
		ID:     "move-to-log-stream",
		Type:   "move",
		From:   "attributes.stream",
		To:     "attributes[\"log.iostream\"]",
		IfExpr: "attributes.stream != nil",
	}
}

// parse body as json and move it to attributes
func expectedMakeJSONParser() Operator {
	regexPattern := `^{.*}$`
	return Operator{
		ID:        "json-parser",
		Type:      "json_parser",
		ParseFrom: "body",
		ParseTo:   "attributes",
		IfExpr:    fmt.Sprintf("body matches '%s'", regexPattern),
	}
}

// copy logs present in body to attributes.original
func expectedMakeCopyBodyToOriginal() Operator {
	return Operator{
		ID:   "copy-body-to-attributes-original",
		Type: "copy",
		From: "body",
		To:   "attributes.original",
	}
}

// look for message in attributes then move it to body
func expectedMakeMoveMessageToBody() Operator {
	return Operator{
		ID:     "move-message-to-body",
		Type:   "move",
		From:   "attributes.message",
		To:     "body",
		IfExpr: "attributes.message != nil",
	}
}

// look for msg if present then move it to body
func expectedMakeMoveMsgToBody() Operator {
	return Operator{
		ID:     "move-msg-to-body",
		Type:   "move",
		From:   "attributes.msg",
		To:     "body",
		IfExpr: "attributes.msg != nil",
	}
}

// set the severity level
func expectedMakeSeverityParser() Operator {
	return Operator{
		ID:        "severity-parser",
		Type:      "severity_parser",
		ParseFrom: "attributes.level",
		IfExpr:    "attributes.level != nil",
	}
}
