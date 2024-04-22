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

func div(nominator, denominator string, selectors ...labelSelector) *exprBuilder {
	for _, s := range selectors {
		nominator = s(nominator)
		denominator = s(denominator)
	}

	eb := &exprBuilder{
		expr: fmt.Sprintf("%s / %s", nominator, denominator),
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

func (eb *exprBuilder) build() string {
	return eb.expr
}
