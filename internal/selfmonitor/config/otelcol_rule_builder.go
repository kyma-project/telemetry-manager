package config

import (
	"fmt"
)

const (
	otelExporterSentMetric          = "otelcol_exporter_sent"
	otelExporterSendFailedMetric    = "otelcol_exporter_send_failed"
	otelExporterQueueSizeMetric     = "otelcol_exporter_queue_size"
	otelExporterQueueCapacityMetric = "otelcol_exporter_queue_capacity"
	otelExporterEnqueueFailedMetric = "otelcol_exporter_enqueue_failed"
	otelReceiverRefusedMetric       = "otelcol_receiver_refused"

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

func (rb otelCollectorRuleBuilder) allDataDroppedExpr() string {
	return unless(or(rb.exporterEnqueueFailedExpr(), rb.exporterDroppedExpr()), rb.exporterSentExpr())
}

func (rb otelCollectorRuleBuilder) someDataDroppedExpr() string {
	return and(or(rb.exporterEnqueueFailedExpr(), rb.exporterDroppedExpr()), rb.exporterSentExpr())
}

func (rb otelCollectorRuleBuilder) exporterSentExpr() string {
	metricName := rb.appendDataType(otelExporterSentMetric)

	return rate(metricName, selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

func (rb otelCollectorRuleBuilder) exporterDroppedExpr() string {
	metricName := rb.appendDataType(otelExporterSendFailedMetric)

	return rate(metricName, selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

func (rb otelCollectorRuleBuilder) exporterEnqueueFailedExpr() string {
	metricName := rb.appendDataType(otelExporterEnqueueFailedMetric)

	return rate(metricName, selectService(rb.serviceName)).
		sumBy(labelPipelineName).
		greaterThan(0).
		build()
}

func (rb otelCollectorRuleBuilder) queueAlmostFullExpr() string {
	//queue size/capacacity do not have data type suffixes unlike other metrics
	nomMetric := otelExporterQueueSizeMetric
	denomMetric := otelExporterQueueCapacityMetric
	return div(nomMetric, denomMetric, ignoringLabelsMatch("data_type"), selectService(rb.serviceName)).
		maxBy(labelPipelineName).
		greaterThan(thresholdQueueAlmostFull).
		build()
}

func (rb otelCollectorRuleBuilder) throttlingExpr() string {
	metricName := rb.appendDataType(otelReceiverRefusedMetric)

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
