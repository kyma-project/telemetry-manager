package config

import (
	"fmt"
)

const (
	metricOTelExporterSent          = "otelcol_exporter_sent"
	metricOTelExporterSendFailed    = "otelcol_exporter_send_failed"
	metricOTelExporterQueueSize     = "otelcol_exporter_queue_size"
	metricOTelExporterQueueCapacity = "otelcol_exporter_queue_capacity"
	metricOTelExporterEnqueueFailed = "otelcol_exporter_enqueue_failed"
	metricOTelReceiverRefused       = "otelcol_receiver_refused"
)

type otelCollectorRuleBuilder struct {
	serviceName string
	dataType    string
	namePrefix  string
}

func (rb otelCollectorRuleBuilder) rules() []Rule {
	return []Rule{
		rb.allDataDroppedRule(),
		rb.someDataDroppedRule(),
		rb.queueAlmostFullRule(),
		rb.throttlingRule(),
	}
}

func (rb otelCollectorRuleBuilder) formatMetricName(baseMetricName string) string {
	return fmt.Sprintf("%s_%s", baseMetricName, rb.dataType)
}

func (rb otelCollectorRuleBuilder) allDataDroppedRule() Rule {
	exporterEnqueueFailedExpr := rate(rb.formatMetricName(metricOTelExporterEnqueueFailed), selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterDroppedExpr := rate(rb.formatMetricName(metricOTelExporterSendFailed), selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterSentExpr := rate(rb.formatMetricName(metricOTelExporterSent), selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	expr := unless(or(exporterEnqueueFailedExpr, exporterDroppedExpr), exporterSentExpr)

	return rb.makeRule(RuleNameGatewayAllDataDropped, expr)
}

func (rb otelCollectorRuleBuilder) someDataDroppedRule() Rule {
	exporterEnqueueFailedExpr := rate(rb.formatMetricName(metricOTelExporterEnqueueFailed), selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterDroppedExpr := rate(rb.formatMetricName(metricOTelExporterSendFailed), selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	exporterSentExpr := rate(rb.formatMetricName(metricOTelExporterSent), selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()

	expr := and(or(exporterEnqueueFailedExpr, exporterDroppedExpr), exporterSentExpr)

	return rb.makeRule(RuleNameGatewaySomeDataDropped, expr)
}

func (rb otelCollectorRuleBuilder) queueAlmostFullRule() Rule {
	expr := div(metricOTelExporterQueueSize, metricOTelExporterQueueCapacity, ignoringLabelsMatch("data_type"), selectService(rb.serviceName)).
		maxBy(labelPipelineName).
		greaterThan(0.8). //nolint:mnd // alert on 80% full
		build()

	return rb.makeRule(RuleNameGatewayQueueAlmostFull, expr)
}

func (rb otelCollectorRuleBuilder) throttlingRule() Rule {
	expr := rate(rb.formatMetricName(metricOTelReceiverRefused), selectService(rb.serviceName)).
		sumBy(labelReceiver).
		greaterThan(0).
		build()

	return rb.makeRule(RuleNameGatewayThrottling, expr)
}

func (rb otelCollectorRuleBuilder) makeRule(baseName, expr string) Rule {
	return Rule{
		Alert: rb.namePrefix + baseName,
		Expr:  expr,
		For:   alertWaitTime,
	}
}
