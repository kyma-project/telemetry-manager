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
	return RuleGroups{
		Groups: []RuleGroup{
			{
				Name: "default",
				Rules: []Rule{
					makeGatewayExporterSentTelemetry(signalTypeMetricPoints),
					makeGatewayExporterSentTelemetry(signalTypeSpans),
					makeGatewayExporterFailedTelemetry(signalTypeMetricPoints),
					makeGatewayExporterFailedTelemetry(signalTypeSpans),
					makeGatewayExporterQueueAlmostFull(),
					makeGatewayReceiverRefusedMetrics(signalTypeMetricPoints),
					makeGatewayReceiverRefusedMetrics(signalTypeSpans),
					makeGatewayExporterEnqueueFailed(signalTypeMetricPoints),
					makeGatewayExporterEnqueueFailed(signalTypeSpans),
				},
			},
		},
	}
}

type signalType string

const (
	signalTypeMetricPoints signalType = "metric_points"
	signalTypeSpans        signalType = "spans"
)

func makeGatewayExporterSentTelemetry(s signalType) Rule {
	metric := fmt.Sprintf("otelcol_exporter_sent_%s", s)
	return Rule{
		Alert: "GatewayExporterSent" + alertNameSuffix(s),
		Expr:  fmt.Sprintf("sum by (exporter) (rate(%s{service=\"%s\"}[1m])) > 0", metric, gatewayName(s)),
	}
}

func makeGatewayExporterFailedTelemetry(s signalType) Rule {
	metric := fmt.Sprintf("otelcol_exporter_send_failed_%s", s)
	return Rule{
		Alert: "GatewayExporterDropped" + alertNameSuffix(s),
		Expr:  fmt.Sprintf("sum by (exporter) (rate(%s{service=\"%s\"}[1m])) > 0", metric, gatewayName(s)),
	}
}

func makeGatewayExporterQueueAlmostFull() Rule {
	return Rule{
		Alert: "GatewayExporterQueueAlmostFull",
		Expr:  "otelcol_exporter_queue_size / otelcol_exporter_queue_capacity > 0.8",
	}
}

func makeGatewayReceiverRefusedMetrics(s signalType) Rule {
	metric := fmt.Sprintf("otelcol_receiver_refused_%s", s)
	return Rule{
		Alert: "GatewayReceiverRefused" + alertNameSuffix(s),
		Expr:  fmt.Sprintf("sum by (receiver) (rate(%s{service=\"%s\"}[1m])) > 0", metric, gatewayName(s)),
	}
}

func makeGatewayExporterEnqueueFailed(s signalType) Rule {
	metric := fmt.Sprintf("otelcol_exporter_enqueue_failed_%s", s)
	return Rule{
		Alert: "GatewayExporterEnqueueFailed" + alertNameSuffix(s),
		Expr:  fmt.Sprintf("sum by (exporter) (rate(%s{service=\"%s\"}[1m])) > 0", metric, gatewayName(s)),
	}
}

func alertNameSuffix(s signalType) string {
	switch s {
	case signalTypeMetricPoints:
		return "MetricPoints"
	case signalTypeSpans:
		return "Spans"
	default:
		return "Telemetry"
	}
}

func gatewayName(s signalType) string {
	switch s {
	case signalTypeMetricPoints:
		return "telemetry-metric-gateway-metrics"
	case signalTypeSpans:
		return "telemetry-trace-gateway-metrics"
	default:
		return ""
	}
}
