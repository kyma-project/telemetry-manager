package agent

import (
	"fmt"

	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const (
	initialInterval = "5s"
	maxInterval     = "30s"
	// Time after which logs will not be discarded. Retrying never stops if value is 0.
	maxElapsedTime        = "300s"
	traceParentExpression = "^[0-9a-f]{2}-(?P<trace_id>[0-9a-f]{32})-(?P<span_id>[0-9a-f]{16})-(?P<trace_flags>[0-9a-f]{2})$"

	attributeKeyLevel       = "level"
	attributeKeyLogLevel    = "log.level"
	attributeKeyStream      = "stream"
	attributeKeyMsg         = "msg"
	attributeKeyMessage     = "message"
	attributeKeyTraceID     = "trace_id"
	attributeKeySpanID      = "span_id"
	attributeKeyTraceFlags  = "trace_flags"
	attributeKeyTraceParent = "traceparent"

	operatorNoop = "noop"
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

	excludeSystemLogAgentPath := makePath("kyma-system", fmt.Sprintf("*%s-*", commonresources.SystemLogAgentName), "collector")
	excludeSystemLogCollectorPath := makePath("kyma-system", fmt.Sprintf("*%s-*", commonresources.SystemLogCollectorName), "collector")
	excludeOtlpLogAgentPath := makePath("kyma-system", fmt.Sprintf("%s-*", otelcollector.LogAgentName), "collector")
	excludeFluentBitPath := makePath("kyma-system", fmt.Sprintf("%s-*", fluentbit.LogAgentName), "fluent-bit")

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
		makeBodyRouter(),
		makeJSONParser(),
	}
	if keepOriginalBody {
		operators = append(operators, makeMoveBodyToLogOriginal())
	}

	operators = append(operators,
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
	)

	return operators
}

// parse the log with containerd parser
func makeContainerParser() Operator {
	return Operator{
		ID:                      "containerd-parser",
		Type:                    Container,
		AddMetadataFromFilePath: ptr.To(true),
		Format:                  "containerd",
	}
}

// move the stream to log.iostream
func makeMoveToLogStream() Operator {
	return Operator{
		ID:     "move-to-log-stream",
		Type:   Move,
		From:   ottlexpr.Attribute(attributeKeyStream),
		To:     ottlexpr.Attribute("log.iostream"),
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeyStream),
	}
}

func makeDropAttributeLogTag() Operator {
	return Operator{
		ID:    "drop-attribute-log-tag",
		Type:  Remove,
		Field: ottlexpr.Attribute("logtag"),
	}
}

func makeBodyRouter() Operator {
	regexPattern := `^{.*}$`

	return Operator{
		ID:      "body-router",
		Type:    Router,
		Default: operatorNoop,
		Routes: []Route{
			{
				Expression: fmt.Sprintf("body matches '%s'", regexPattern),
				Output:     "json-parser",
			},
		},
	}
}

// parse body as json and move it to attributes
func makeJSONParser() Operator {
	return Operator{
		ID:        "json-parser",
		Type:      JsonParser,
		ParseFrom: "body",
		ParseTo:   "attributes",
	}
}

// move logs present in body to attributes.log.original
func makeMoveBodyToLogOriginal() Operator {
	return Operator{
		ID:   "move-body-to-attributes-log-original",
		Type: Move,
		From: "body",
		To:   ottlexpr.Attribute("log.original"),
	}
}

// look for message in attributes then move it to body
func makeMoveMessageToBody() Operator {
	return Operator{
		ID:     "move-message-to-body",
		Type:   Move,
		From:   ottlexpr.Attribute(attributeKeyMessage),
		To:     "body",
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeyMessage),
	}
}

// look for msg if present then move it to body
func makeMoveMsgToBody() Operator {
	return Operator{
		ID:     "move-msg-to-body",
		Type:   Move,
		From:   ottlexpr.Attribute(attributeKeyMsg),
		To:     "body",
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeyMsg),
	}
}

// parse severity from level attribute
func makeSeverityParserFromLevel() Operator {
	return Operator{
		ID:        "parse-level",
		Type:      SeverityParser,
		ParseFrom: ottlexpr.Attribute(attributeKeyLevel),
		IfExpr:    ottlexpr.AttributeIsNotNil(attributeKeyLevel),
	}
}

// Remove level attribute after parsing severity
func makeRemoveLevel() Operator {
	return Operator{
		ID:     "remove-level",
		Type:   Remove,
		Field:  ottlexpr.Attribute(attributeKeyLevel),
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeyLevel),
	}
}

// parse severity from log level attribute
func makeSeverityParserFromLogLevel() Operator {
	return Operator{
		ID:        "parse-log-level",
		Type:      SeverityParser,
		ParseFrom: ottlexpr.Attribute(attributeKeyLogLevel),
		IfExpr:    ottlexpr.AttributeIsNotNil(attributeKeyLogLevel),
	}
}

// Remove log level attribute after parsing severity
func makeRemoveLogLevel() Operator {
	return Operator{
		ID:     "remove-log-level",
		Type:   Remove,
		Field:  ottlexpr.Attribute(attributeKeyLogLevel),
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeyLogLevel),
	}
}

func makeTraceRouter() Operator {
	return Operator{
		ID:      "trace-router",
		Type:    Router,
		Default: operatorNoop,
		Routes: []Route{
			{
				Expression: ottlexpr.AttributeIsNotNil(attributeKeyTraceID),
				Output:     "trace-parser",
			},
			{
				Expression: ottlexpr.JoinWithAnd(ottlexpr.AttributeIsNotNil(attributeKeyTraceParent), fmt.Sprintf("%s matches '%s'", ottlexpr.Attribute(attributeKeyTraceParent), traceParentExpression)),
				Output:     "trace-parent-parser",
			},
		},
	}
}

// set the severity level
func makeTraceParser() Operator {
	return Operator{
		ID:     "trace-parser",
		Type:   TraceParser,
		Output: "remove-trace-id",
		TraceID: OperatorAttribute{
			ParseFrom: ottlexpr.Attribute(attributeKeyTraceID),
		},
		SpanID: OperatorAttribute{
			ParseFrom: ottlexpr.Attribute(attributeKeySpanID),
		},
		TraceFlags: OperatorAttribute{
			ParseFrom: ottlexpr.Attribute(attributeKeyTraceFlags),
		},
	}
}

func makeTraceParentParser() Operator {
	return Operator{
		ID:        "trace-parent-parser",
		Type:      RegexParser,
		Regex:     traceParentExpression,
		ParseFrom: ottlexpr.Attribute(attributeKeyTraceParent),
		Output:    "remove-trace-parent",
		Trace: TraceAttribute{
			TraceID: OperatorAttribute{
				ParseFrom: ottlexpr.Attribute(attributeKeyTraceID),
			},
			SpanID: OperatorAttribute{
				ParseFrom: ottlexpr.Attribute(attributeKeySpanID),
			},
			TraceFlags: OperatorAttribute{
				ParseFrom: ottlexpr.Attribute(attributeKeyTraceFlags),
			},
		},
	}
}

func makeRemoveTraceParent() Operator {
	return Operator{
		ID:    "remove-trace-parent",
		Type:  Remove,
		Field: ottlexpr.Attribute(attributeKeyTraceParent),
	}
}

func makeRemoveTraceID() Operator {
	return Operator{
		ID:     "remove-trace-id",
		Type:   Remove,
		Field:  ottlexpr.Attribute(attributeKeyTraceID),
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeyTraceID),
	}
}

func makeRemoveSpanID() Operator {
	return Operator{
		ID:     "remove-span-id",
		Type:   Remove,
		Field:  ottlexpr.Attribute(attributeKeySpanID),
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeySpanID),
	}
}

func makeRemoveTraceFlags() Operator {
	return Operator{
		ID:     "remove-trace-flags",
		Type:   Remove,
		Field:  ottlexpr.Attribute(attributeKeyTraceFlags),
		IfExpr: ottlexpr.AttributeIsNotNil(attributeKeyTraceFlags),
	}
}

// The noop operator is required because of router operator, an entry that does not match any of the routes is dropped and not processed further, to avoid that we add a noop operator as default route
func makeNoop() Operator {
	return Operator{
		ID:   operatorNoop,
		Type: Noop,
	}
}
