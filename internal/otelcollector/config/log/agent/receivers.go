package agent

import (
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"

	"k8s.io/utils/ptr"
)

func makeReceivers(logpipelines []telemetryv1alpha1.LogPipeline) Receivers {
	return Receivers{
		FileLog: &FileLog{
			Exclude: []string{
				"/var/log/pods/kyma-system_telemetry-log-agent*/*/*.log",
				"/var/log/pods/kyma-system_telemetry-fluent-bit*/*/*.log",
			},
			Include:         []string{"/var/log/pods/*/*/*.log"},
			IncludeFileName: false,
			IncludeFilePath: true,
			StartAt:         "beginning",
			Storage:         "file_storage",
			Operators:       makeOperators(logpipelines),
		},
	}
}

func makeOperators(logPipelines []telemetryv1alpha1.LogPipeline) []Operator {
	keepOriginalBody := false
	for _, logPipeline := range logPipelines {
		if *logPipeline.Spec.Input.Application.KeepOriginalBody {
			keepOriginalBody = true
		}
	}

	if keepOriginalBody {
		return []Operator{
			makeContainerParser(),
			makeMoveToLogStream(),
			makeJSONParser(),
			makeCopyBodyToOriginal(),
			makeMoveMessageToBody(),
			makeSeverityParser(),
		}
	}
	return []Operator{
		makeContainerParser(),
		makeMoveToLogStream(),
		makeJSONParser(),
		makeMoveMessageToBody(),
		makeSeverityParser(),
	}
}

func makeContainerParser() Operator {
	return Operator{
		ID:                      "containerd-parser",
		Type:                    "container",
		AddMetadataFromFilePath: ptr.To(true),
		Format:                  "containerd",
	}
}

func makeMoveToLogStream() Operator {
	return Operator{
		ID:     "move-to-log-stream",
		Type:   "move",
		From:   "attributes.stream",
		IfExpr: "attributes.stream != nil",
		To:     "attributes[\"log.iostream\"]",
	}
}

func makeJSONParser() Operator {
	regexPattern := `^{(?:\\s*"(?:[^"\\]|\\.)*"\\s*:\\s*(?:null|true|false|\\d+|\\d*\\.\\d+|"(?:[^"\\]|\\.)*"|\\{[^{}]*\\}|\\[[^\\[\\]]*\\])\\s*,?)*\\s*}$`

	return Operator{
		ID:        "json-parser",
		Type:      "json_parser",
		IfExpr:    fmt.Sprintf("body matches '%s'", regexPattern),
		ParseFrom: "body",
		ParseTo:   "attributes",
	}
}

func makeCopyBodyToOriginal() Operator {
	return Operator{
		ID:   "copy-body-to-attributes-original",
		Type: "copy",
		From: "body",
		To:   "attributes.original",
	}
}

func makeMoveMessageToBody() Operator {
	return Operator{
		ID:     "move-message-to-body",
		Type:   "move",
		IfExpr: "attributes.message != nil",
		From:   "attributes.message",
		To:     "body",
	}
}

func makeSeverityParser() Operator {
	return Operator{
		ID:        "severity-parser",
		Type:      "severity_parser",
		IfExpr:    "attributes.level != nil",
		ParseFrom: "attributes.level",
	}
}
