package alertrules

import (
	"time"
)

const (
	// OTEL Collector rule names. Note that the actual full names will be prefixed with Metric or Trace
	RuleNameGatewayExporterSentData        = "GatewayExporterSentData"
	RuleNameGatewayExporterDroppedData     = "GatewayExporterDroppedData"
	RuleNameGatewayExporterQueueAlmostFull = "GatewayExporterQueueAlmostFull"
	RuleNameGatewayExporterEnqueueFailed   = "GatewayExporterEnqueueFailed"
	RuleNameGatewayReceiverRefusedData     = "GatewayReceiverRefusedData"

	// Fluent Bit rule names
	RuleNameLogAgentExporterSentLogs    = "LogAgentExporterSentLogs"
	RuleNameLogAgentReceiverReadLogs    = "LogAgentReceiverReadLogs"
	RuleNameLogAgentExporterDroppedLogs = "LogAgentExporterDroppedLogs"
	RuleNameLogAgentBufferInUse         = "LogAgentBufferInUse"
	RuleNameLogAgentBufferFull          = "LogAgentBufferFull"

	LabelService = "service"

	// OTel Collector rule labels
	LabelExporter = "exporter"
	LabelReceiver = "receiver"
)

// RuleGroups is a set of rule groups that are typically exposed in a file.
type RuleGroups struct {
	Groups []RuleGroup `yaml:"groups"`
}

// RuleGroup is a list of sequentially evaluated alerting rules.
type RuleGroup struct {
	Name  string `yaml:"name"`
	Rules []Rule `yaml:"rules"`
}

// Rule describes an alerting rule.
type Rule struct {
	Alert       string            `yaml:"alert,omitempty"`
	Expr        string            `yaml:"expr"`
	For         time.Duration     `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

func MakeRules() RuleGroups {
	var rules []Rule

	metricRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "metric_points",
		serviceName: "telemetry-metric-gateway-metrics",
		namePrefix:  RuleNamePrefix(MetricPipeline),
	}
	rules = append(rules, metricRuleBuilder.rules()...)

	traceRuleBuilder := otelCollectorRuleBuilder{
		dataType:    "spans",
		serviceName: "telemetry-trace-collector-metrics",
		namePrefix:  RuleNamePrefix(TracePipeline),
	}
	rules = append(rules, traceRuleBuilder.rules()...)

	logRuleBuilder := fluentBitRuleBuilder{}
	rules = append(rules, logRuleBuilder.rules()...)

	return RuleGroups{
		Groups: []RuleGroup{
			{
				Name:  "default",
				Rules: rules,
			},
		},
	}
}

func RuleNamePrefix(t PipelineType) string {
	if t == TracePipeline {
		return "Trace"
	}

	return "Metric"
}
