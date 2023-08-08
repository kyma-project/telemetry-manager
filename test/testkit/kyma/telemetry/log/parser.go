//go:build e2e

package log

import (
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type Parser struct {
	name   string
	parser string
}

func NewParser(name, parser string) *Parser {
	return &Parser{
		name:   name,
		parser: parser,
	}
}

func (p *Parser) K8sObject() *telemetry.LogParser {
	return &telemetry.LogParser{
		ObjectMeta: k8smeta.ObjectMeta{
			Name: p.name,
		},
		Spec: telemetry.LogParserSpec{
			Parser: p.parser,
		},
	}
}
