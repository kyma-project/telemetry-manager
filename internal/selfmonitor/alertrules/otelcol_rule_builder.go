package alertrules

import (
	"fmt"
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

func (rb otelCollectorRuleBuilder) exporterSentRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_sent_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterSentData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumByPipelineName().
			greaterThan(0).
			build(),
	}
}

func (rb otelCollectorRuleBuilder) exporterDroppedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_send_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterDroppedData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumByPipelineName().
			greaterThan(0).
			build(),
	}
}

func (rb otelCollectorRuleBuilder) exporterQueueAlmostFullRule() Rule {
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterQueueAlmostFull,
		Expr: div("otelcol_exporter_queue_size", "otelcol_exporter_queue_capacity", selectService(rb.serviceName)).
			greaterThan(0.8).
			build(),
	}
}

func (rb otelCollectorRuleBuilder) exporterEnqueueFailedRule() Rule {
	metric := fmt.Sprintf("otelcol_exporter_enqueue_failed_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayExporterEnqueueFailed,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumByPipelineName().
			greaterThan(0).
			build(),
	}
}

func (rb otelCollectorRuleBuilder) receiverRefusedRule() Rule {
	metric := fmt.Sprintf("otelcol_receiver_refused_%s", rb.dataType)
	return Rule{
		Alert: rb.namePrefix + RuleNameGatewayReceiverRefusedData,
		Expr: rate(metric, selectService(rb.serviceName)).
			sumBy(labelReceiver).
			greaterThan(0).
			build(),
	}
}
