package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogParser struct {
	name   string
	parser string
}

func NewLogParser(name, parser string) *LogParser {
	return &LogParser{
		name:   name,
		parser: parser,
	}
}

func (p *LogParser) K8sObject() *telemetryv1alpha1.LogParser {
	return &telemetryv1alpha1.LogParser{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.name,
		},
		Spec: telemetryv1alpha1.LogParserSpec{
			Parser: p.parser,
		},
	}
}
