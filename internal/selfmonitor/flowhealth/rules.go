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
	metricRuleBuilder := newRuleBuilder(signalTypeMetricPoints)
	traceRuleBuilder := newRuleBuilder(signalTypeSpans)

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

type telemetryDataType string

const (
	signalTypeMetricPoints telemetryDataType = "metric_points"
	signalTypeSpans        telemetryDataType = "spans"
)

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
	alertNamePrefix string
	serviceName     string
	dataType        telemetryDataType
}

func newRuleBuilder(dataType telemetryDataType) ruleBuilder {
	alertNamePrefix := "MetricGateway"
	serviceName := "telemetry-metric-gateway-metrics"

	if dataType == signalTypeSpans {
		alertNamePrefix = "TraceGateway"
		serviceName = "telemetry-trace-gateway-metrics"
	}

	return ruleBuilder{
		alertNamePrefix: alertNamePrefix,
		dataType:        dataType,
		serviceName:     serviceName,
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
		Alert: rb.alertNamePrefix + alertNameExporterSentData,
		Expr: rate(metric, selectLabel(serviceLabelKey, rb.serviceName)).
			sumBy(exporterLabelKey).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) exporterDroppedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_send_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.alertNamePrefix + alertNameExporterDroppedData,
		Expr: rate(metric, selectLabel(serviceLabelKey, rb.serviceName)).
			sumBy(exporterLabelKey).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) exporterQueueAlmostFullRule() Rule {
	return Rule{
		Alert: rb.alertNamePrefix + alertNameExporterQueueAlmostFull,
		Expr: div("otelcol_exporter_queue_size", "otelcol_exporter_queue_capacity", selectLabel(serviceLabelKey, rb.serviceName)).
			greaterThan(0.8).
			build(),
	}
}

func (rb ruleBuilder) exporterEnqueueFailedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_enqueue_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.alertNamePrefix + alertNameExporterEnqueueFailed,
		Expr: rate(metric, selectLabel(serviceLabelKey, rb.serviceName)).
			sumBy(exporterLabelKey).
			greaterThan(0).
			build(),
	}
}

func (rb ruleBuilder) receiverRefusedRule() Rule {
	metric := fmt.Sprintf("otelcol_receiver_refused_%s", rb.dataType)
	return Rule{
		Alert: rb.alertNamePrefix + alertNameReceiverRefusedData,
		Expr: rate(metric, selectLabel(serviceLabelKey, rb.serviceName)).
			sumBy(receiverLabelKey).
			greaterThan(0).
			build(),
	}
}
