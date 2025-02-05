package agent

import (
	"fmt"

	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

func makeReceivers(logpipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) Receivers {
	excludeLogAgentLogs := fmt.Sprintf("/var/log/pods/%s_%s*/*/*.log", opts.AgentNamespace, otelcollector.LogAgentName)
	excludeFluentBitLogs := fmt.Sprintf("/var/log/pods/%s_%s*/*/*.log", opts.AgentNamespace, fluentbit.LogAgentName)

	return Receivers{
		FileLog: &FileLog{
			Exclude: []string{
				excludeLogAgentLogs,
				excludeFluentBitLogs,
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
		ID:   "move-to-log-stream",
		Type: "move",
		From: "attributes.stream",
		To:   "attributes[\"log.iostream\"]",
	}
}

func makeJSONParser() Operator {
	return Operator{
		ID:        "json-parser",
		Type:      "json_parser",
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
		ID:   "move-message-to-body",
		Type: "move",
		From: "attributes.message",
		To:   "body",
	}
}

func makeSeverityParser() Operator {
	return Operator{
		ID:        "severity-parser",
		Type:      "severity_parser",
		ParseFrom: "attributes.level",
	}
}
