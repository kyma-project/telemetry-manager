package logagent

import (
	"fmt"

	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
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

func fileLogReceiverConfig(lp *telemetryv1alpha1.LogPipeline, collectAgentLogs bool) *FileLogReceiver {
	excludePath := createExcludePath(lp.Spec.Input.Application, collectAgentLogs)

	includePath := createIncludePath(lp.Spec.Input.Application)

	return &FileLogReceiver{
		Exclude:         excludePath,
		Include:         includePath,
		IncludeFileName: ptr.To(false),
		IncludeFilePath: ptr.To(true),
		StartAt:         "beginning",
		Storage:         "file_storage",
		RetryOnFailure: common.RetryOnFailure{
			Enabled:         true,
			InitialInterval: initialInterval,
			MaxInterval:     maxInterval,
			MaxElapsedTime:  maxElapsedTime,
		},
		Operators: makeOperators(lp),
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

func createExcludePath(application *telemetryv1alpha1.LogPipelineApplicationInput, collectAgentLogs bool) []string {
	var excludePath, excludeContainers []string

	if !collectAgentLogs {
		excludePath = append(excludePath, makePath("kyma-system", fmt.Sprintf("%s-*", fluentbit.LogAgentName), "fluent-bit"))
		excludePath = append(excludePath, makePath("kyma-system", fmt.Sprintf("%s-*", otelcollector.LogAgentName), "collector"))
	}

	excludeSystemLogAgentPath := makePath("kyma-system", fmt.Sprintf("*%s-*", commonresources.SystemLogAgentName), "collector")
	excludeSystemLogCollectorPath := makePath("kyma-system", fmt.Sprintf("*%s-*", commonresources.SystemLogCollectorName), "collector")

	excludePath = append(excludePath, excludeSystemLogAgentPath, excludeSystemLogCollectorPath)

	var excludeNamespaces []string

	if application != nil {
		excludeNamespaces = append(excludeNamespaces, application.Namespaces.Exclude...)
		excludeContainers = append(excludeContainers, application.Containers.Exclude...)
	}
	if application == nil || (!application.Namespaces.System && len(application.Namespaces.Include) == 0 && len(application.Namespaces.Exclude) == 0) {
		excludeNamespaces = namespaces.System()
	}

	//if application == nil || (!application.Namespaces.System && len(application.Namespaces.Include) == 0 && len(application.Namespaces.Exclude) == 0) {
	//	systemLogPath := []string{}
	//	for _, ns := range namespaces.System() {
	//		systemLogPath = append(systemLogPath, fmt.Sprintf("/var/log/pods/%s_*/*/*.log", ns))
	//	}
	//
	//	excludePath = append(excludePath, systemLogPath...)
	//}

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

func makeOperators(lp *telemetryv1alpha1.LogPipeline) []Operator {
	keepOriginalBody := *lp.Spec.Input.Application.KeepOriginalBody

	operators := []Operator{
		makeContainerParser(),
		makeMoveToLogStream(),
		makeDropAttributeLogTag(),
		makeBodyRouter(),
		makeJSONParser(),
	}
	if keepOriginalBody {
		operators = append(operators, makeMoveBodyToLogOriginal())
	} else {
		operators = append(operators, makeRemoveBody())
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
		From:   common.Attribute(attributeKeyStream),
		To:     common.Attribute("log.iostream"),
		IfExpr: common.AttributeIsNotNil(attributeKeyStream),
	}
}

func makeDropAttributeLogTag() Operator {
	return Operator{
		ID:    "drop-attribute-log-tag",
		Type:  Remove,
		Field: common.Attribute("logtag"),
	}
}

func makeBodyRouter() Operator {
	regexPattern := `^{.*}$`

	// If body is not a JSON document, then skip all operators as they are all based on a parsed record and go to noop
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
		To:   common.Attribute("log.original"),
	}
}

// remove body attribute
func makeRemoveBody() Operator {
	return Operator{
		ID:    "remove-body",
		Type:  Remove,
		Field: "body",
	}
}

// look for message in attributes then move it to body
func makeMoveMessageToBody() Operator {
	return Operator{
		ID:     "move-message-to-body",
		Type:   Move,
		From:   common.Attribute(attributeKeyMessage),
		To:     "body",
		IfExpr: common.AttributeIsNotNil(attributeKeyMessage),
	}
}

// look for msg if present then move it to body
func makeMoveMsgToBody() Operator {
	return Operator{
		ID:     "move-msg-to-body",
		Type:   Move,
		From:   common.Attribute(attributeKeyMsg),
		To:     "body",
		IfExpr: common.AttributeIsNotNil(attributeKeyMsg),
	}
}

// parse severity from level attribute
func makeSeverityParserFromLevel() Operator {
	return Operator{
		ID:        "parse-level",
		Type:      SeverityParser,
		ParseFrom: common.Attribute(attributeKeyLevel),
		IfExpr:    common.AttributeIsNotNil(attributeKeyLevel),
	}
}

// Remove level attribute after parsing severity
func makeRemoveLevel() Operator {
	return Operator{
		ID:     "remove-level",
		Type:   Remove,
		Field:  common.Attribute(attributeKeyLevel),
		IfExpr: common.AttributeIsNotNil(attributeKeyLevel),
	}
}

// parse severity from log level attribute
func makeSeverityParserFromLogLevel() Operator {
	return Operator{
		ID:        "parse-log-level",
		Type:      SeverityParser,
		ParseFrom: common.Attribute(attributeKeyLogLevel),
		IfExpr:    common.AttributeIsNotNil(attributeKeyLogLevel),
	}
}

// Remove log level attribute after parsing severity
func makeRemoveLogLevel() Operator {
	return Operator{
		ID:     "remove-log-level",
		Type:   Remove,
		Field:  common.Attribute(attributeKeyLogLevel),
		IfExpr: common.AttributeIsNotNil(attributeKeyLogLevel),
	}
}

func makeTraceRouter() Operator {
	return Operator{
		ID:      "trace-router",
		Type:    Router,
		Default: operatorNoop,
		Routes: []Route{
			{
				Expression: common.AttributeIsNotNil(attributeKeyTraceID),
				Output:     "trace-parser",
			},
			{
				Expression: common.JoinWithAnd(common.AttributeIsNotNil(attributeKeyTraceParent), fmt.Sprintf("%s matches '%s'", common.Attribute(attributeKeyTraceParent), traceParentExpression)),
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
			ParseFrom: common.Attribute(attributeKeyTraceID),
		},
		SpanID: OperatorAttribute{
			ParseFrom: common.Attribute(attributeKeySpanID),
		},
		TraceFlags: OperatorAttribute{
			ParseFrom: common.Attribute(attributeKeyTraceFlags),
		},
	}
}

func makeTraceParentParser() Operator {
	return Operator{
		ID:        "trace-parent-parser",
		Type:      RegexParser,
		Regex:     traceParentExpression,
		ParseFrom: common.Attribute(attributeKeyTraceParent),
		Output:    "remove-trace-parent",
		Trace: TraceAttribute{
			TraceID: OperatorAttribute{
				ParseFrom: common.Attribute(attributeKeyTraceID),
			},
			SpanID: OperatorAttribute{
				ParseFrom: common.Attribute(attributeKeySpanID),
			},
			TraceFlags: OperatorAttribute{
				ParseFrom: common.Attribute(attributeKeyTraceFlags),
			},
		},
	}
}

func makeRemoveTraceParent() Operator {
	return Operator{
		ID:    "remove-trace-parent",
		Type:  Remove,
		Field: common.Attribute(attributeKeyTraceParent),
	}
}

func makeRemoveTraceID() Operator {
	return Operator{
		ID:     "remove-trace-id",
		Type:   Remove,
		Field:  common.Attribute(attributeKeyTraceID),
		IfExpr: common.AttributeIsNotNil(attributeKeyTraceID),
	}
}

func makeRemoveSpanID() Operator {
	return Operator{
		ID:     "remove-span-id",
		Type:   Remove,
		Field:  common.Attribute(attributeKeySpanID),
		IfExpr: common.AttributeIsNotNil(attributeKeySpanID),
	}
}

func makeRemoveTraceFlags() Operator {
	return Operator{
		ID:     "remove-trace-flags",
		Type:   Remove,
		Field:  common.Attribute(attributeKeyTraceFlags),
		IfExpr: common.AttributeIsNotNil(attributeKeyTraceFlags),
	}
}

// The noop operator is required because of router operator, an entry that does not match any of the routes is dropped and not processed further, to avoid that we add a noop operator as default route
func makeNoop() Operator {
	return Operator{
		ID:   operatorNoop,
		Type: Noop,
	}
}
