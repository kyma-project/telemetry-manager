package config

import (
	"fmt"
	"strconv"
	"strings"
)

const defaultRateDuration = "5m"

type exprBuilder struct {
	expr string
}

type labelSelector func(string) string

func selectService(serviceName string) labelSelector {
	return func(metric string) string {
		return fmt.Sprintf("%s{%s=\"%s\"}", metric, labelService, serviceName)
	}
}

func instant(metric string, selectors ...labelSelector) *exprBuilder {
	for _, s := range selectors {
		metric = s(metric)
	}

	eb := &exprBuilder{
		expr: metric,
	}

	return eb
}

func rate(metric string, selectors ...labelSelector) *exprBuilder {
	for _, s := range selectors {
		metric = s(metric)
	}

	eb := &exprBuilder{
		expr: fmt.Sprintf("rate(%s[%s])", metric, defaultRateDuration),
	}

	return eb
}

func (eb *exprBuilder) sumBy(labels ...string) *exprBuilder {
	eb.expr = fmt.Sprintf("sum by (%s) (%s)", strings.Join(labels, ","), eb.expr)
	return eb
}

func (eb *exprBuilder) maxBy(labels ...string) *exprBuilder {
	eb.expr = fmt.Sprintf("max by (%s) (%s)", strings.Join(labels, ","), eb.expr)
	return eb
}

func (eb *exprBuilder) greaterThan(value float64) *exprBuilder {
	eb.expr = fmt.Sprintf("%s > %s", eb.expr, strconv.FormatFloat(value, 'f', -1, 64))
	return eb
}

func (eb *exprBuilder) equal(value float64) *exprBuilder {
	eb.expr = fmt.Sprintf("%s == %s", eb.expr, strconv.FormatFloat(value, 'f', -1, 64))
	return eb
}

func (eb *exprBuilder) build() string {
	return eb.expr
}

// Logical/set binary operators
// https://prometheus.io/docs/prometheus/latest/querying/operators/#logical-set-binary-operators

func and(exprs ...string) string {
	return strings.Join(wrapInParentheses(exprs), " and ")
}

func or(exprs ...string) string {
	return strings.Join(wrapInParentheses(exprs), " or ")
}

func unless(exprs ...string) string {
	return strings.Join(wrapInParentheses(exprs), " unless ")
}

func wrapInParentheses(input []string) []string {
	wrapped := make([]string, len(input))
	for i, str := range input {
		wrapped[i] = fmt.Sprintf("(%s)", str)
	}

	return wrapped
}
