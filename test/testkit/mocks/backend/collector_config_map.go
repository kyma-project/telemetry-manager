package backend

import (
	"bytes"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

type collectorConfigMapBuilder struct {
	name             string
	namespace        string
	exportedFilePath string
	signalType       SignalType
	certs            *testutils.ServerCerts
	oidc             *OIDCConfig
	mtls             bool
}

func newCollectorConfigMap(name, namespace, path string, signalType SignalType, certs *testutils.ServerCerts, oidc *OIDCConfig, mtls bool) *collectorConfigMapBuilder {
	return &collectorConfigMapBuilder{
		name:             name,
		namespace:        namespace,
		exportedFilePath: path,
		signalType:       signalType,
		certs:            certs,
		oidc:             oidc,
		mtls:             mtls,
	}
}

const unifiedConfigTemplate = `
{{- if .OIDCEnabled }}
extensions:
  oidc:
    issuer_url: "{{ .IssuerURL }}"
    audience: "{{ .Audience }}"
{{- end }}
receivers:
  {{- if .FluentBit }}
  fluentforward:
    endpoint: localhost:8006
  {{- end }}
  otlp:
    protocols:
      grpc:
        {{- if .UseTLS }}
        tls:
          cert_pem: "{{ .CertPem }}"
          key_pem: "{{ .KeyPem }}"
          {{- if .MTLS }}
          client_ca_file: {{ .CaFilePath }}
          {{- end }}
        {{- end }}
        {{- if .OIDCEnabled }}
        auth:
          authenticator: oidc
        {{- end }}
        endpoint: ${MY_POD_IP}:4317
      http:
        endpoint: ${MY_POD_IP}:4318
exporters:
  file:
    path: {{ .FilePath }}
service:
  telemetry:
    logs:
      level: "info"
  pipelines:
    {{ .SignalType }}:
      receivers:
        - otlp
        {{- if .FluentBit }}
        - fluentforward
        {{- end }}
      exporters:
        - file
  {{- if .OIDCEnabled }}
  extensions:
    - oidc
  {{- end }}
`

func (cm *collectorConfigMapBuilder) Name() string {
	return cm.name
}

func (cm *collectorConfigMapBuilder) K8sObject() *corev1.ConfigMap {
	configTemplate := unifiedConfigTemplate

	// Prepare template data
	signal := string(cm.signalType)
	if cm.signalType == SignalTypeLogsOTel {
		signal = "logs"
	}

	isFluent := cm.signalType == SignalTypeLogsFluentBit
	useTLS := cm.certs != nil && !isFluent
	oidcEnabled := cm.oidc != nil

	tplData := struct {
		FilePath    string
		SignalType  string
		CertPem     string
		KeyPem      string
		CaFilePath  string
		UseTLS      bool
		MTLS        bool
		FluentBit   bool
		OIDCEnabled bool
		IssuerURL   string
		Audience    string
	}{
		FilePath:    cm.exportedFilePath,
		SignalType:  signal,
		CertPem:     "",
		KeyPem:      "",
		CaFilePath:  "",
		UseTLS:      useTLS,
		FluentBit:   isFluent,
		OIDCEnabled: false,
		IssuerURL:   "",
		Audience:    "",
		MTLS:        cm.mtls,
	}

	if oidcEnabled {
		tplData.IssuerURL = cm.oidc.issuerURL
		tplData.Audience = cm.oidc.audience
		tplData.OIDCEnabled = true
	}

	data := make(map[string]string)

	if useTLS {
		tplData.CertPem = strings.ReplaceAll(cm.certs.ServerCertPem.String(), "\n", "\\n")
		tplData.KeyPem = strings.ReplaceAll(cm.certs.ServerKeyPem.String(), "\n", "\\n")
		tplData.CaFilePath = "/etc/collector/ca.crt"

		data["ca.crt"] = cm.certs.CaCertPem.String()
	}

	// Render template using text/template
	tpl, err := template.New("collector").Parse(configTemplate)
	if err != nil {
		panic(err) // Template parsing should not fail
	}

	var buf bytes.Buffer

	err = tpl.Execute(&buf, tplData)
	if err != nil {
		panic(err) // Template execution should not fail
	}

	config := buf.String()

	data["config.yaml"] = config

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: data,
	}
}
