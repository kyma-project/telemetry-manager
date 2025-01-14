package config

import (
	"fmt"
)

const (
	metricOtelCollectorExporterSent          = "otelcol_exporter_sent"
	metricOtelCollectorExporterSendFailed    = "otelcol_exporter_send_failed"
	metricOtelCollectorExporterQueueSize     = "otelcol_exporter_queue_size"
	metricOtelCollectorExporterQueueCapacity = "otelcol_exporter_queue_capacity"
	metricOtelCollectorExporterEnqueueFailed = "otelcol_exporter_enqueue_failed"
	metricOtelCollectorReceiverRefused       = "otelcol_receiver_refused"
)

type otelCollectorRuleBuilder struct {
	serviceName string
	dataType    string
	namePrefix  string
}

func (rb otelCollectorRuleBuilder) rules() []Rule {
	return []Rule{
		rb.exporterSentRule(),
		rb.exporterDroppedRule(),
		rb.exporterQueueAlmostFullRule(),
		rb.exporterEnqueueFailedRule(),
		rb.receiverRefusedRule(),
	}
}

func (rb otelCollectorRuleBuilder) formatMetricName(baseMetricName string) string {
	return fmt.Sprintf("%s_%s", baseMetricName, rb.dataType)
}

func (rb otelCollectorRuleBuilder) exporterSentRule() Rule {
	metric := rb.formatMetricName(metricOtelCollectorExporterSent)

	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterSentData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb otelCollectorRuleBuilder) exporterDroppedRule() Rule {
	metric := rb.formatMetricName(metricOtelCollectorExporterSendFailed)

	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterDroppedData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb otelCollectorRuleBuilder) exporterQueueAlmostFullRule() Rule {
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterQueueAlmostFull,
		Expr: div(metricOtelCollectorExporterQueueSize, metricOtelCollectorExporterQueueCapacity, ignoringLabelsMatch("data_type"), selectService(rb.serviceName)).
			maxBy(labelPipelineName).
			greaterThan(0.8). //nolint:mnd // alert on 80% full
			build(),
	}
}

func (rb otelCollectorRuleBuilder) exporterEnqueueFailedRule() Rule {
	metric := rb.formatMetricName(metricOtelCollectorExporterEnqueueFailed)

	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterEnqueueFailed,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(labelPipelineName).
			greaterThan(0).
			build(),
	}
}

func (rb otelCollectorRuleBuilder) receiverRefusedRule() Rule {
	metric := rb.formatMetricName(metricOtelCollectorReceiverRefused)

	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayReceiverRefusedData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(labelReceiver).
			greaterThan(0).
			build(),
	}
}
