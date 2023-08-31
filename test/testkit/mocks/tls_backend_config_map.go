package mocks

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TLSBackendConfigMap struct {
	name             string
	namespace        string
	exportedFilePath string
	signalType       SignalType
	certPem          string
	keyPem           string
	caPem            string
}

func NewTLSBackendConfigMap(name, namespace, path, certPem, keyPem, caPem string, signalType SignalType) *TLSBackendConfigMap {
	return &TLSBackendConfigMap{
		name:             name,
		namespace:        namespace,
		exportedFilePath: path,
		signalType:       signalType,
		certPem:          certPem,
		keyPem:           keyPem,
		caPem:            caPem,
	}
}

const tlsConfigTemplate = `receivers:
  otlp:
    protocols:
      grpc:
        tls:
          cert_pem: "{{ CERT_PEM }}"
          key_pem: "{{ KEY_PEM }}"
          client_ca_file: {{ CA_FILE_PATH }}
      http: {}
exporters:
  file:
    path: {{ FILEPATH }}
  logging:
    loglevel: debug
service:
  telemetry:
    logs:
      level: "debug"
  pipelines:
    {{ SIGNAL_TYPE }}:
      receivers:
        - otlp
      exporters:
        - file
        - logging`

func (cm *TLSBackendConfigMap) Name() string {
	return cm.name
}

func (cm *TLSBackendConfigMap) K8sObject() *corev1.ConfigMap {
	certPem := strings.ReplaceAll(cm.certPem, "\n", "\\n")
	keyPem := strings.ReplaceAll(cm.keyPem, "\n", "\\n")

	config := strings.Replace(tlsConfigTemplate, "{{ FILEPATH }}", cm.exportedFilePath, 1)
	config = strings.Replace(config, "{{ SIGNAL_TYPE }}", string(cm.signalType), 1)
	config = strings.Replace(config, "{{ CERT_PEM }}", certPem, 1)
	config = strings.Replace(config, "{{ KEY_PEM }}", keyPem, 1)
	config = strings.Replace(config, "{{ CA_FILE_PATH }}", "/etc/collector/ca.crt", 1)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: map[string]string{"config.yaml": config, "ca.crt": cm.caPem},
	}
}
