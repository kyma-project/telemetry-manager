package config

import (
	"fmt"
)

const (
	//OTel Collector metrics

	// following metrics should be used with data type suffixes (metric points, spans, etc.)
	otelExporterSent          = "otelcol_exporter_sent"
	otelExporterSendFailed    = "otelcol_exporter_send_failed"
	otelExporterEnqueueFailed = "otelcol_exporter_enqueue_failed"
	otelReceiverRefused       = "otelcol_receiver_refused"

	// queue size/capacacity metrics do not have data type suffixes unlike other metrics
	otelExporterQueueSize     = "otelcol_exporter_queue_size"
	otelExporterQueueCapacity = "otelcol_exporter_queue_capacity"

	thresholdQueueAlmostFull = 0.8
)

type otelCollectorRuleBuilder struct {
	serviceName string
	dataType    string
	namePrefix  string
}

func (rb otelCollectorRuleBuilder) rules() []Rule {
	return []Rule{
		rb.makeRule(RuleNameGatewayAllDataDropped, rb.allDataDroppedExpr()),
		rb.makeRule(RuleNameGatewaySomeDataDropped, rb.someDataDroppedExpr()),
		rb.makeRule(RuleNameGatewayQueueAlmostFull, rb.queueAlmostFullExpr()),
		rb.makeRule(RuleNameGatewayThrottling, rb.throttlingExpr()),
	}
}

// Checks if all data is dropped due to a full buffer or exporter issues, with nothing successfully sent.
func (rb otelCollectorRuleBuilder) allDataDroppedExpr() string {
	return unless(
		or(rb.exporterEnqueueFailedExpr(), rb.exporterDroppedExpr()),
		rb.exporterSentExpr(),
	)
}

// Checks if some data is dropped while some is still successfully sent.
func (rb otelCollectorRuleBuilder) someDataDroppedExpr() string {
	return and(
		or(rb.exporterEnqueueFailedExpr(), rb.exporterDroppedExpr()),
		rb.exporterSentExpr(),
	)
}

// Check if the exporter drop rate is greater than 0.
func (rb otelCollectorRuleBuilder) exporterSentExpr() string {
	metricName := rb.appendDataType(otelExporterSent)

	return rate(metricName, selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

// Check if the exporter send rate is greater than 0.
func (rb otelCollectorRuleBuilder) exporterDroppedExpr() string {
	metricName := rb.appendDataType(otelExporterSendFailed)

	return rate(metricName, selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

// Check if the exporter enqueue failure rate is greater than 0.
func (rb otelCollectorRuleBuilder) exporterEnqueueFailedExpr() string {
	metricName := rb.appendDataType(otelExporterEnqueueFailed)

	return rate(metricName, selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

// Check if the queue is almost full.
func (rb otelCollectorRuleBuilder) queueAlmostFullExpr() string {
	nomMetric := otelExporterQueueSize
	denomMetric := otelExporterQueueCapacity
	return div(nomMetric, denomMetric, ignoringLabelsMatch("data_type"), selectService(rb.serviceName)).
		maxBy(labelPipelineName).
		greaterThan(thresholdQueueAlmostFull).
		build()
}

// Check if the receiver data refusal rate is greater than 0.
func (rb otelCollectorRuleBuilder) throttlingExpr() string {
	metricName := rb.appendDataType(otelReceiverRefused)

	return rate(metricName, selectService(rb.serviceName)).
		sumBy(labelReceiver).
		greaterThan(0).
		build()
}

func (rb otelCollectorRuleBuilder) appendDataType(baseMetricName string) string {
	return fmt.Sprintf("%s_%s", baseMetricName, rb.dataType)
}

func (rb otelCollectorRuleBuilder) makeRule(baseName, expr string) Rule {
	return Rule{
		Alert: rb.namePrefix + baseName,
		Expr:  expr,
		For:   alertWaitTime,
	}
}
