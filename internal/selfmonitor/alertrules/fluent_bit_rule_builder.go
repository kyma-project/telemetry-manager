package alertrules

type fluentBitRuleBuilder struct {
	serviceName string
	namePrefix  string
}

func (rb fluentBitRuleBuilder) rules() []Rule {
	return []Rule{
		rb.exporterSentRule(),
		rb.receiverReadRule(),
		rb.exporterDroppedRule(),
		rb.bufferInUseRule(),
		rb.bufferFullRule(),
	}
}

func (rb fluentBitRuleBuilder) exporterSentRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentExporterSentLogs,
		Expr: rate("fluentbit_output_bytes_total", selectService(rb.serviceName)).
			sumBy(LabelExporter).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) receiverReadRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentReceiverReadLogs,
		Expr: rate("fluentbit_input_bytes_total", selectService(rb.serviceName)).
			sumBy(LabelExporter).
			greaterThan(0).
			build(),
	}
}

func (rb fluentBitRuleBuilder) exporterDroppedRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentExporterDroppedLogs,
		Expr: div("fluentbit_output_retries_failed_total", "otelcol_exporter_queue_capacity", selectService(rb.serviceName)).
			greaterThan(0.8).
			build(),
	}
}

func (rb fluentBitRuleBuilder) bufferInUseRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentBufferInUse,
		Expr: rate("telemetry_fsbuffer_usage_bytes", selectService(rb.serviceName)).
			sumBy(LabelExporter).
			greaterThan(300000000).
			build(),
	}
}

func (rb fluentBitRuleBuilder) bufferFullRule() Rule {
	return Rule{
		Alert: RuleNameLogAgentBufferFull,
		Expr: rate("telemetry_fsbuffer_usage_bytes", selectService(rb.serviceName)).
			sumBy(LabelReceiver).
			greaterThan(900000000).
			build(),
	}
}
