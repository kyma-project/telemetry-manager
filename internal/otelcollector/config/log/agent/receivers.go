package agent

import (
	"fmt"

	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const (
	initialInterval = "5s"
	maxInterval     = "30s"
	// Time after which logs will not be discarded. Retrying never stops if value is 0.
	maxElapsedTime        = "300s"
	traceParentExpression = "^[0-9a-f]{2}-(?P<trace_id>[0-9a-f]{32})-(?P<span_id>[0-9a-f]{16})-(?P<trace_flags>[0-9a-f]{2})$"
	attributeTraceID      = "attributes.trace_id"
	attributeSpanID       = "attributes.span_id"
	attributeTraceFlags   = "attributes.trace_flags"
	attributeTraceParent  = "attributes.traceparent"
)

func makeFileLogReceiver(logpipeline telemetryv1alpha1.LogPipeline) *FileLog {
	excludePath := createExcludePath(logpipeline.Spec.Input.Application)

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
	var includePath []string

	includeNamespaces := []string{"*"}
	includeContainers := []string{"*"}

	if application != nil {
		if len(application.Namespaces.Include) > 0 {
			includeNamespaces = application.Namespaces.Include
		}

		if len(application.Containers.Include) > 0 {
			includeContainers = application.Containers.Include
		}
	}

	for _, ns := range includeNamespaces {
		for _, container := range includeContainers {
			includePath = append(includePath, makePath(ns, "*", container))
		}
	}

	return includePath
}

func createExcludePath(application *telemetryv1alpha1.LogPipelineApplicationInput) []string {
	var excludePath []string

	var excludeContainers []string

	var excludeNamespaces []string

	excludeSystemLogAgentPath := makePath("kyma-system", fmt.Sprintf("*%s*", commonresources.SystemLogAgentName), "*")
	excludeSystemLogCollectorPath := makePath("kyma-system", fmt.Sprintf("*%s*", commonresources.SystemLogCollectorName), "*")
	excludeOtlpLogAgentPath := makePath("kyma-system", fmt.Sprintf("%s*", otelcollector.LogAgentName), "*")
	excludeFluentBitPath := makePath("kyma-system", "telemetry-fluent-bit*", "*")

	excludePath = append(excludePath, excludeSystemLogAgentPath, excludeSystemLogCollectorPath, excludeOtlpLogAgentPath, excludeFluentBitPath)

	if application == nil || !application.Namespaces.System {
		systemLogPath := []string{}
		for _, ns := range namespaces.System() {
			systemLogPath = append(systemLogPath, fmt.Sprintf("/var/log/pods/%s_*/*/*.log", ns))
		}

		excludePath = append(excludePath, systemLogPath...)
	}

	if application != nil {
		if len(application.Namespaces.Exclude) > 0 {
			excludeNamespaces = append(excludeNamespaces, application.Namespaces.Exclude...)
		}

		if len(application.Containers.Exclude) > 0 {
			excludeContainers = append(excludeContainers, application.Containers.Exclude...)
		}
	}

	for _, ns := range excludeNamespaces {
		excludePath = append(excludePath, fmt.Sprintf("/var/log/pods/%s_*/*/*.log", ns))
	}

	for _, container := range excludeContainers {
		excludePath = append(excludePath, fmt.Sprintf("/var/log/pods/*_*/%s/*.log", container))
	}

	return excludePath
}

func makePath(namespace, pod, container string) string {
	pathPattern := "/var/log/pods/%s_%s/%s/*.log"
	return fmt.Sprintf(pathPattern, namespace, pod, container)
}

func makeOperators(logPipeline telemetryv1alpha1.LogPipeline) []Operator {
	keepOriginalBody := *logPipeline.Spec.Input.Application.KeepOriginalBody

	operators := []Operator{
		makeContainerParser(),
		makeMoveToLogStream(),
		makeDropAttributeLogTag(),
		makeJSONParser(),
	}
	if keepOriginalBody {
		operators = append(operators, makeMoveBodyToLogOriginal())
	}

	operators = append(operators,
		makeMoveMessageToBody(),
		makeMoveMsgToBody(),
		makeSeverityParser(),
		makeTraceRouter(),
		makeTraceParentParser(),
		makeTraceParser(),
		makeRemoveTraceParent(),
		makeRemoveTraceID(),
		makeRemoveSpanID(),
		makeRemoveTraceFlags(),
		makeNoop(),
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

// move logs present in body to attributes.log.original
func makeMoveBodyToLogOriginal() Operator {
	return Operator{
		ID:   "move-body-to-attributes-log-original",
		Type: "move",
		From: "body",
		To:   "attributes[\"log.original\"]",
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

func makeTraceRouter() Operator {
	return Operator{
		ID:      "trace-router",
		Type:    "router",
		Default: "noop",
		Routes: []Router{
			{
				Expression: fmt.Sprintf("%s != nil", attributeTraceID),
				Output:     "trace-parser",
			},
			{
				Expression: fmt.Sprintf("%s == nil and %s != nil and attributes.traceparent matches '%s'", attributeTraceID, attributeTraceParent, traceParentExpression),
				Output:     "trace-parent-parser",
			},
		},
	}
}

// set the severity level
func makeTraceParser() Operator {
	return Operator{
		ID:     "trace-parser",
		Type:   "trace_parser",
		Output: "remove-trace-id",
		TraceID: OperatorAttribute{
			ParseFrom: attributeTraceID,
		},
		SpanID: OperatorAttribute{
			ParseFrom: attributeSpanID,
		},
		TraceFlags: OperatorAttribute{
			ParseFrom: attributeTraceFlags,
		},
	}
}

func makeTraceParentParser() Operator {
	return Operator{
		ID:        "trace-parent-parser",
		Type:      "regex_parser",
		Regex:     traceParentExpression,
		ParseFrom: attributeTraceParent,
		Output:    "remove-trace-parent",
		Trace: TraceAttribute{
			TraceID: OperatorAttribute{
				ParseFrom: attributeTraceID,
			},
			SpanID: OperatorAttribute{
				ParseFrom: attributeSpanID,
			},
			TraceFlags: OperatorAttribute{
				ParseFrom: attributeTraceFlags,
			},
		},
	}
}

func makeRemoveTraceParent() Operator {
	return Operator{
		ID:     "remove-trace-parent",
		Type:   "remove",
		Field:  attributeTraceParent,
		Output: "remove-trace-id",
	}
}

func makeRemoveTraceID() Operator {
	return Operator{
		ID:     "remove-trace-id",
		Type:   "remove",
		Field:  attributeTraceID,
		Output: "remove-span-id",
	}
}

func makeRemoveSpanID() Operator {
	return Operator{
		ID:     "remove-span-id",
		Type:   "remove",
		Field:  attributeSpanID,
		IfExpr: "attributes.span_id != nil",
		Output: "remove-trace-flags",
	}
}

func makeRemoveTraceFlags() Operator {
	return Operator{
		ID:     "remove-trace-flags",
		Type:   "remove",
		IfExpr: "attributes.trace_flags != nil",
		Field:  attributeTraceFlags,
	}
}

// The noop operator is required because of router operator, an entry that does not match any of the routes is dropped and not processed further, to avoid that we add a noop operator as default route
func makeNoop() Operator {
	return Operator{
		ID:   "noop",
		Type: "noop",
	}
}
