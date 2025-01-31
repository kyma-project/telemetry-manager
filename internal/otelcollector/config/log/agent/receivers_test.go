package agent

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
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
			receivers := makeReceivers(tc.pipelines)
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
		{
			ID:                      "containerd-parser",
			Type:                    "container",
			AddMetadataFromFilePath: ptr.To(true),
			Format:                  "containerd",
		},
		{
			ID:     "move-to-log-stream",
			Type:   "move",
			From:   "attributes.stream",
			To:     "attributes[\"log.iostream\"]",
			IfExpr: "attributes.stream != nil",
		},
		{
			ID:        "json-parser",
			Type:      "json_parser",
			IfExpr:    "body matches '^{(?:\\\\s*\"(?:[^\"\\\\]|\\\\.)*\"\\\\s*:\\\\s*(?:null|true|false|\\\\d+|\\\\d*\\\\.\\\\d+|\"(?:[^\"\\\\]|\\\\.)*\"|\\\\{[^{}]*\\\\}|\\\\[[^\\\\[\\\\]]*\\\\])\\\\s*,?)*\\\\s*}$'",
			ParseFrom: "body",
			ParseTo:   "attributes",
		},
		{
			ID:   "copy-body-to-attributes-original",
			Type: "copy",
			From: "body",
			To:   "attributes.original",
		},
		{
			ID:     "move-message-to-body",
			Type:   "move",
			From:   "attributes.message",
			To:     "body",
			IfExpr: "attributes.message != nil",
		},
		{
			ID:        "severity-parser",
			Type:      "severity_parser",
			IfExpr:    "attributes.level != nil",
			ParseFrom: "attributes.level",
		},
	}
}

func makeExpectedOperatorsWithoutKeepOringinalBody() []Operator {
	return []Operator{
		{
			ID:                      "containerd-parser",
			Type:                    "container",
			AddMetadataFromFilePath: ptr.To(true),
			Format:                  "containerd",
		},
		{
			ID:     "move-to-log-stream",
			Type:   "move",
			From:   "attributes.stream",
			To:     "attributes[\"log.iostream\"]",
			IfExpr: "attributes.stream != nil",
		},
		{
			ID:        "json-parser",
			Type:      "json_parser",
			IfExpr:    "body matches '^{(?:\\\\s*\"(?:[^\"\\\\]|\\\\.)*\"\\\\s*:\\\\s*(?:null|true|false|\\\\d+|\\\\d*\\\\.\\\\d+|\"(?:[^\"\\\\]|\\\\.)*\"|\\\\{[^{}]*\\\\}|\\\\[[^\\\\[\\\\]]*\\\\])\\\\s*,?)*\\\\s*}$'",
			ParseFrom: "body",
			ParseTo:   "attributes",
		},

		{
			ID:     "move-message-to-body",
			Type:   "move",
			From:   "attributes.message",
			To:     "body",
			IfExpr: "attributes.message != nil",
		},
		{
			ID:        "severity-parser",
			Type:      "severity_parser",
			IfExpr:    "attributes.level != nil",
			ParseFrom: "attributes.level",
		},
	}
}
