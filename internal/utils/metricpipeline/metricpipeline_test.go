package metricpipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractBackendPort(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		port     string
	}{
		{
			name:     "with scheme",
			endpoint: "https://sample.endpoint.com:443",
			port:     "443",
		},
		{
			name:     "with scheme and path",
			endpoint: "https://sample.endpoint.com:443/api/test",
			port:     "443",
		},
		{
			name:     "without scheme",
			endpoint: "sample.endpoint.com:9090",
			port:     "9090",
		},
		{
			name:     "without scheme and with path",
			endpoint: "sample.endpoint.com:8080/api/test",
			port:     "8080",
		},
		{
			name:     "with grpc scheme",
			endpoint: "grpc://sample.endpoint.com:4317",
			port:     "4317",
		},
		{
			name:     "with scheme and without port",
			endpoint: "https://sample.endpoint.com",
			port:     "443",
		},
		{
			name:     "without scheme and without port",
			endpoint: "sample.endpoint.com",
			port:     "80",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			port, err := extractPort(test.endpoint)
			require.NoError(t, err)
			require.Equal(t, test.port, port)
		})
	}
}
