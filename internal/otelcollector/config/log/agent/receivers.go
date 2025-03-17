package agent

import (
	"fmt"

	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const (
	initialInterval = "5s"
	maxInterval     = "30s"
	// Time after which logs will not be discarded. Retrying never stops if value is 0.
	maxElapsedTime = "300s"
)

func makeFileLogReceiver(logpipeline telemetryv1alpha1.LogPipeline, opts BuildOptions) *FileLog {
	excludeLogAgentLogs := fmt.Sprintf("/var/log/pods/%s_%s*/*/*.log", opts.AgentNamespace, otelcollector.LogAgentName)
	excludeFluentBitLogs := fmt.Sprintf("/var/log/pods/%s_%s*/*/*.log", opts.AgentNamespace, fluentbit.LogAgentName)
	excludeSystemLogCollectorLogs := fmt.Sprintf("/var/log/pods/%s_*%s*/*/*.log", opts.AgentNamespace, commonresources.SystemLogCollectorName)
	excludeSystemLogAgentLogs := fmt.Sprintf("/var/log/pods/%s_*%s*/*/*.log", opts.AgentNamespace, commonresources.SystemLogAgentName)

	excludePath := createExcludePath(logpipeline.Spec.Input.Application)
	excludePath = append(excludePath,
		excludeLogAgentLogs,
		excludeFluentBitLogs,
		excludeSystemLogCollectorLogs,
		excludeSystemLogAgentLogs)

	includePath := createIncludePath(logpipeline.Spec.Input.Application)

	return &FileLog{
		Exclude:         excludePath,
		Include:         includePath,
		IncludeFileName: ptr.To(false),
		IncludeFilePath: ptr.To(true),
		StartAt:         "beginning",
		Storage:         "file_storage",
		RetryOnFailure: config.RetryOnFailure{
			Enabled:         true,
			InitialInterval: initialInterval,
			MaxInterval:     maxInterval,
			MaxElapsedTime:  maxElapsedTime,
		},
		Operators: makeOperators(logpipeline),
	}
}

func createIncludePath(application *telemetryv1alpha1.LogPipelineApplicationInput) []string {
	if application == nil || application.Namespaces.Include == nil && !application.Namespaces.System {
		return []string{"/var/log/pods/*/*/*.log"}
	}

	includeNamespacePath := []string{}
	for _, ns := range application.Namespaces.Include {
		includeNamespacePath = append(includeNamespacePath, fmt.Sprintf("/var/log/pods/%s_*/*/*.log", ns))
	}

	if application.Namespaces.System {
		return makeSystemLogPath()
	}

	return includeNamespacePath
}

func createExcludePath(application *telemetryv1alpha1.LogPipelineApplicationInput) []string {
	if application == nil || application.Namespaces.Exclude == nil && !application.Namespaces.System {
		return makeSystemLogPath()
	}

	excludeNamespacePath := []string{}
	if !application.Namespaces.System {
		excludeNamespacePath = append(excludeNamespacePath, makeSystemLogPath()...)
	}

	for _, ns := range application.Namespaces.Exclude {
		excludeNamespacePath = append(excludeNamespacePath, fmt.Sprintf("/var/log/pods/%s_*/*/*.log", ns))
	}

	return excludeNamespacePath
}

func makeSystemLogPath() []string {
	systemLogPath := []string{}
	for _, ns := range namespaces.System() {
		systemLogPath = append(systemLogPath, fmt.Sprintf("/var/log/pods/%s_*/*/*.log", ns))
	}

	return systemLogPath
}

func makeOperators(logPipeline telemetryv1alpha1.LogPipeline) []Operator {
	keepOriginalBody := false

	if *logPipeline.Spec.Input.Application.KeepOriginalBody {
		keepOriginalBody = true
	}

	operators := []Operator{
		makeContainerParser(),
		makeMoveToLogStream(),
		makeDropAttributeLogTag(),
		makeJSONParser(),
	}
	if keepOriginalBody {
		operators = append(operators, makeCopyBodyToOriginal())
	}

	operators = append(operators,
		makeMoveMessageToBody(),
		makeMoveMsgToBody(),
		makeSeverityParser(),
	)

	return operators
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

func makeDropAttributeLogTag() Operator {
	return Operator{
		ID:    "drop-attribute-log-tag",
		Type:  "remove",
		Field: "attributes[\"logtag\"]",
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
