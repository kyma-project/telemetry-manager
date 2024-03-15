package flowhealth

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
	metricRuleBuilder := newRuleBuilder(FlowTypeMetrics)
	traceRuleBuilder := newRuleBuilder(FlowTypeTraces)

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
	serviceLabelKey  = "service"
	exporterLabelKey = "exporter"
	receiverLabelKey = "receiver"

	alertNameExporterSentData        = "ExporterSentData"
	alertNameExporterDroppedData     = "ExporterDroppedData"
	alertNameExporterQueueAlmostFull = "ExporterQueueAlmostFull"
	alertNameExporterEnqueueFailed   = "ExporterEnqueueFailed"
	alertNameReceiverRefusedData     = "ReceiverRefusedData"
)

type ruleBuilder struct {
	serviceName   string
	dataType      string
	nameDecorator ruleNameDecorator
}

type ruleNameDecorator func(string) string

var traceRuleNameDecorator = func(name string) string {
	return "TraceGateway" + name
}

var metricRuleNameDecorator = func(name string) string {
	return "MetricGateway" + name
}

func newRuleBuilder(t FlowType) ruleBuilder {
	serviceName := "telemetry-metric-gateway-metrics"
	dataType := "metric_points"
	nameDecorator := metricRuleNameDecorator

	if t == FlowTypeTraces {
		serviceName = "telemetry-trace-collector-metrics"
		dataType = "spans"
		nameDecorator = traceRuleNameDecorator
	}

	return ruleBuilder{
		dataType:      dataType,
		serviceName:   serviceName,
		nameDecorator: nameDecorator,
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
		Alert: rb.nameDecorator(alertNameExporterSentData),
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(exporterLabelKey).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) exporterDroppedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_send_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.nameDecorator(alertNameExporterDroppedData),
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(exporterLabelKey).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) exporterQueueAlmostFullRule() Rule {
	return Rule{
		Alert: rb.nameDecorator(alertNameExporterQueueAlmostFull),
		Expr: div("otelcol_exporter_queue_size", "otelcol_exporter_queue_capacity", selectService(rb.serviceName)).
			greaterThan(0.8).
			build(),
	}
}

func (rb ruleBuilder) exporterEnqueueFailedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_enqueue_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.nameDecorator(alertNameExporterEnqueueFailed),
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(exporterLabelKey).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) receiverRefusedRule() Rule {
	metric := fmt.Sprintf("otelcol_receiver_refused_%s", rb.dataType)
	return Rule{
		Alert: rb.nameDecorator(alertNameReceiverRefusedData),
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(receiverLabelKey).
			greaterThan(0).
			build(),
	}
}
