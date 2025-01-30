package agent

import (
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

	receivers := makeReceivers()

	require.Equal(t, expectedExcludePaths, receivers.FileLog.Exclude)
	require.Equal(t, expectedIncludePaths, receivers.FileLog.Include)
	require.Equal(t, false, receivers.FileLog.IncludeFileName)
	require.Equal(t, true, receivers.FileLog.IncludeFilePath)

	expectedOperators := makeExpectedOperators()

	operators := receivers.FileLog.Operators
	require.Len(t, operators, 6)
	require.Equal(t, expectedOperators, operators)
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
