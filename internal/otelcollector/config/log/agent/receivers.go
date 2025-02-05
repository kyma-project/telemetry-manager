package agent

import (
	"fmt"

	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const (
	initialInterval = "5s"
	maxInterval     = "30s"
	// Time after which logs will not be discarded. Retrying never stops if value is 0.
	maxElapsedTime = "300s"
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
			RetryOnFailure: config.RetryOnFailure{
				Enabled:         true,
				InitialInterval: initialInterval,
				MaxInterval:     maxInterval,
				MaxElapsedTime:  maxElapsedTime,
			},
			Operators: makeOperators(logpipelines),
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
			makeMoveMsgToBody(),
			makeSeverityParser(),
		}
	}

	return []Operator{
		makeContainerParser(),
		makeMoveToLogStream(),
		makeJSONParser(),
		makeMoveMessageToBody(),
		makeMoveMsgToBody(),
		makeSeverityParser(),
	}
}

// parse the log with containerd parser
func makeContainerParser() Operator {
	return Operator{
		ID:                      "containerd-parser",
		Type:                    "container",
		AddMetadataFromFilePath: ptr.To(true),
		Format:                  "containerd",
	}
}

// move the stream to log.iostream
func makeMoveToLogStream() Operator {
	return Operator{
		ID:     "move-to-log-stream",
		Type:   "move",
		From:   "attributes.stream",
		To:     "attributes[\"log.iostream\"]",
		IfExpr: "attributes.stream != nil",
	}
}

// parse body as json and move it to attributes
func makeJSONParser() Operator {
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
func makeCopyBodyToOriginal() Operator {
	return Operator{
		ID:   "copy-body-to-attributes-original",
		Type: "copy",
		From: "body",
		To:   "attributes.original",
	}
}

// look for message in attributes then move it to body
func makeMoveMessageToBody() Operator {
	return Operator{
		ID:     "move-message-to-body",
		Type:   "move",
		From:   "attributes.message",
		To:     "body",
		IfExpr: "attributes.message != nil",
	}
}

// look for msg if present then move it to body
func makeMoveMsgToBody() Operator {
	return Operator{
		ID:     "move-msg-to-body",
		Type:   "move",
		From:   "attributes.msg",
		To:     "body",
		IfExpr: "attributes.msg != nil",
	}
}

// set the severity level
func makeSeverityParser() Operator {
	return Operator{
		ID:        "severity-parser",
		Type:      "severity_parser",
		ParseFrom: "attributes.level",
		IfExpr:    "attributes.level != nil",
	}
}
