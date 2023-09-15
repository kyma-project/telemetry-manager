package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogParserBuilder struct {
	randSource rand.Source

	name   string
	parser string
}

func NewLogParsersBuilder() *LogParserBuilder {
	return &LogParserBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
	}
}

func (b *LogParserBuilder) WithName(name string) *LogParserBuilder {
	b.name = name
	return b
}

func (b *LogParserBuilder) WithParser(parser string) *LogParserBuilder {
	b.parser = parser
	return b
}

func (b *LogParserBuilder) Build() telemetryv1alpha1.LogParser {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}
	return telemetryv1alpha1.LogParser{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: telemetryv1alpha1.LogParserSpec{
			Parser: b.parser,
		},
	}
}
