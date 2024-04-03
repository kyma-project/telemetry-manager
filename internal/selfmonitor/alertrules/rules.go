package alertrules

import (
	"fmt"
	"time"
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
	metricRuleBuilder := newRuleBuilder(MetricPipeline)
	traceRuleBuilder := newRuleBuilder(TracePipeline)

	ruleBuilders := []ruleBuilder{metricRuleBuilder, traceRuleBuilder}
	var rules []Rule
	for _, rb := range ruleBuilders {
		rules = append(rules, rb.rules()...)
	}

	return RuleGroups{
		Groups: []RuleGroup{
			{
				Name:  "default",
				Rules: rules,
			},
		},
	}
}

const (
	LabelService  = "service"
	LabelExporter = "exporter"
	LabelReceiver = "receiver"

	RuleNameGatewayExporterSentData        = "GatewayExporterSentData"
	RuleNameGatewayExporterDroppedData     = "GatewayExporterDroppedData"
	RuleNameGatewayExporterQueueAlmostFull = "GatewayExporterQueueAlmostFull"
	RuleNameGatewayExporterEnqueueFailed   = "GatewayExporterEnqueueFailed"
	RuleNameGatewayReceiverRefusedData     = "GatewayReceiverRefusedData"
)

func RuleNamePrefix(t PipelineType) string {
	if t == TracePipeline {
		return "Trace"
	}

	return "Metric"
}

type ruleBuilder struct {
	serviceName string
	dataType    string
	namePrefix  string
}

func newRuleBuilder(t PipelineType) ruleBuilder {
	serviceName := "telemetry-metric-gateway-metrics"
	dataType := "metric_points"

	if t == TracePipeline {
		serviceName = "telemetry-trace-collector-metrics"
		dataType = "spans"
	}

	return ruleBuilder{
		dataType:    dataType,
		serviceName: serviceName,
		namePrefix:  RuleNamePrefix(t),
	}
}

func (rb ruleBuilder) rules() []Rule {
	return []Rule{
		rb.exporterSentRule(),
		rb.exporterDroppedRule(),
		rb.exporterQueueAlmostFullRule(),
		rb.exporterEnqueueFailedRule(),
		rb.receiverRefusedRule(),
	}
}

func (rb ruleBuilder) exporterSentRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_sent_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterSentData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(LabelExporter).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) exporterDroppedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_send_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterDroppedData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(LabelExporter).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) exporterQueueAlmostFullRule() Rule {
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterQueueAlmostFull,
		Expr: div("otelcol_exporter_queue_size", "otelcol_exporter_queue_capacity", selectService(rb.serviceName)).
			greaterThan(0.8).
			build(),
	}
}

func (rb ruleBuilder) exporterEnqueueFailedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_enqueue_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterEnqueueFailed,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(LabelExporter).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) receiverRefusedRule() Rule {
	metric := fmt.Sprintf("otelcol_receiver_refused_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayReceiverRefusedData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(LabelReceiver).
			greaterThan(0).
			build(),
	}
}
