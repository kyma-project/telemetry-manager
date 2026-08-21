package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteAsPassthrough(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bare host:port gets prefixed",
			input:    "otlp.server:4317",
			expected: "passthrough:///otlp.server:4317",
		},
		{
			name:     "http:// stripped then prefixed",
			input:    "http://otlp.server:4317",
			expected: "passthrough:///otlp.server:4317",
		},
		{
			name:     "https:// stripped then prefixed",
			input:    "https://otlp.server:4317",
			expected: "passthrough:///otlp.server:4317",
		},
		{
			name:     "already passthrough:/// left unchanged",
			input:    "passthrough:///otlp.server:4317",
			expected: "passthrough:///otlp.server:4317",
		},
		{
			name:     "whitespace trimmed before check",
			input:    "  otlp.server:4317  ",
			expected: "passthrough:///otlp.server:4317",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteAsPassthrough([]byte(tt.input))
			require.Equal(t, tt.expected, string(result))
		})
	}
}
