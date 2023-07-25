package log

import (
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (p *Parser) K8sObject() *v1alpha1.LogParser {
	return &v1alpha1.LogParser{
		ObjectMeta: k8smeta.ObjectMeta{
			Name: p.name,
		},
		Spec: v1alpha1.LogParserSpec{
			Parser: p.parser,
		},
	}
}
