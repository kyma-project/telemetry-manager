package metric

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func mustMarshalMetrics(md pmetric.Metrics) []byte {
	var marshaler pmetric.JSONMarshaler
	bytes, err := marshaler.MarshalMetrics(md)
	if err != nil {
		panic(err)
	}
	return bytes
}
