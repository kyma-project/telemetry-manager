package backend

import (
	"strings"
	"testing"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestCollectorConfigMapGeneration(t *testing.T) {
	// Prepare certs once for cases that need them
	serverCerts, _, err := testutils.NewCertBuilder("backend", "ns").Build()
	if err != nil {
		t.Fatalf("failed to build test certs: %v", err)
	}

	tests := []struct {
		name              string
		signalType        SignalType
		certs             *testutils.ServerCerts
		exportedFilePath  string
		expectContains    []string
		expectNotContains []string
		expectHasCA       bool
		oidc              *OIDCConfig
		mtls              bool
	}{
		{
			name:             "OTLP default (no certs)",
			signalType:       SignalTypeTraces,
			certs:            nil,
			exportedFilePath: "/traces/otlp-data.jsonl",
			expectContains: []string{
				"/traces/otlp-data.jsonl",
				"pipelines:",
				"traces:",
			},
			expectNotContains: nil,
			expectHasCA:       false,
		},
		{
			name:             "TLS enabled for traces",
			signalType:       SignalTypeTraces,
			certs:            serverCerts,
			exportedFilePath: "/traces/otlp-data.jsonl",
			expectContains: []string{
				"/traces/otlp-data.jsonl",
				"client_ca_file: /etc/collector/ca.crt",
			},
			expectNotContains: nil,
			expectHasCA:       true,
			mtls:              true,
		},
		{
			name:             "FluentBit logs should include fluentforward and ignore certs",
			signalType:       SignalTypeLogsFluentBit,
			certs:            serverCerts,
			exportedFilePath: "/logs/otlp-data.jsonl",
			expectContains: []string{
				"fluentforward:",
			},
			expectNotContains: []string{"client_ca_file", "cert_pem", "key_pem"},
			expectHasCA:       false,
		},
		{
			name:             "Logs OTEL maps to 'logs' pipeline and supports TLS",
			signalType:       SignalTypeLogsOTel,
			certs:            serverCerts,
			exportedFilePath: "/logs-otel/otlp-data.jsonl",
			expectContains: []string{
				"pipelines:",
				"logs:", // mapped from logs-otel
				"client_ca_file: /etc/collector/ca.crt",
			},
			expectNotContains: nil,
			expectHasCA:       true,
			mtls:              true,
		},
		{
			name:             "OIDC enabled for traces with MTLS",
			signalType:       SignalTypeTraces,
			certs:            serverCerts,
			exportedFilePath: "/traces/otlp-data.jsonl",
			oidc: &OIDCConfig{
				issuerURL: "https://issuer.example",
				audience:  "example-aud",
			},
			expectContains: []string{
				"extensions:",
				"oidc:",
				"issuer_url: \"https://issuer.example\"",
				"audience: \"example-aud\"",
				"auth:",
				"authenticator: oidc",
				"- oidc",
				"client_ca_file: /etc/collector/ca.crt",
			},
			expectNotContains: nil,
			expectHasCA:       true,
			mtls:              true,
		},
		{
			name:             "OIDC enabled for traces without MTLS",
			signalType:       SignalTypeTraces,
			certs:            serverCerts,
			exportedFilePath: "/traces/otlp-data.jsonl",
			oidc: &OIDCConfig{
				issuerURL: "https://issuer.example",
				audience:  "example-aud",
			},
			expectContains: []string{
				"extensions:",
				"oidc:",
				"issuer_url: \"https://issuer.example\"",
				"audience: \"example-aud\"",
				"auth:",
				"authenticator: oidc",
				"- oidc",
			},
			expectNotContains: []string{
				"client_ca_file: /etc/collector/ca.crt",
			},
			expectHasCA: true,
			mtls:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cm := newCollectorConfigMap("test-name", "test-ns", tc.exportedFilePath, tc.signalType, tc.certs, tc.oidc, tc.mtls)
			obj := cm.K8sObject()
			if obj == nil {
				t.Fatalf("expected non-nil ConfigMap from builder")
			}
			if obj.ObjectMeta.Name != "test-name" {
				t.Fatalf("unexpected name: got %s", obj.ObjectMeta.Name)
			}
			if obj.ObjectMeta.Namespace != "test-ns" {
				t.Fatalf("unexpected namespace: got %s", obj.ObjectMeta.Namespace)
			}

			data := obj.Data
			config, ok := data["config.yaml"]
			if !ok {
				t.Fatalf("config.yaml missing from ConfigMap.Data")
			}

			// Basic contains/doesn't contain checks
			for _, want := range tc.expectContains {
				if !strings.Contains(config, want) {
					t.Fatalf("expected config to contain %q but it did not. config:\n%s", want, config)
				}
			}
			for _, notWant := range tc.expectNotContains {
				if strings.Contains(config, notWant) {
					t.Fatalf("expected config NOT to contain %q but it did. config:\n%s", notWant, config)
				}
			}

			// CA presence checks in Data
			_, hasCA := data["ca.crt"]
			if tc.expectHasCA && !hasCA {
				t.Fatalf("expected ca.crt to be present in ConfigMap.Data")
			}
			if !tc.expectHasCA && hasCA {
				t.Fatalf("did not expect ca.crt in ConfigMap.Data but it was present")
			}

			// If CA expected, ensure content matches the cert builder's CA PEM
			if tc.expectHasCA {
				if data["ca.crt"] != tc.certs.CaCertPem.String() {
					t.Fatalf("ca.crt content mismatch: expected %q, got %q", tc.certs.CaCertPem.String(), data["ca.crt"])
				}

				// Ensure cert/key were escaped into the config when TLS template used
				escapedCert := strings.ReplaceAll(tc.certs.ServerCertPem.String(), "\n", "\\n")
				if !strings.Contains(config, escapedCert) {
					t.Fatalf("expected escaped server cert to be present in config")
				}
			}
		})
	}
}
