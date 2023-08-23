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
		certPem:          strings.ReplaceAll(certPem, "\n", "\\n"),
		keyPem:           strings.ReplaceAll(keyPem, "\n", "\\n"),
		caPem:            strings.ReplaceAll(caPem, "\n", "\\n"),
	}
}

const tlsConfigTemplate = `receivers:
  otlp:
    protocols:
      grpc:
        tls:
          cert_pem: "{{ CERT_PEM }}"
          key_pem: "{{ KEY_PEM }}"
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
	config := strings.Replace(tlsConfigTemplate, "{{ FILEPATH }}", cm.exportedFilePath, 1)
	config = strings.Replace(config, "{{ SIGNAL_TYPE }}", string(cm.signalType), 1)
	config = strings.Replace(config, "{{ CERT_PEM }}", cm.certPem, 1)
	config = strings.Replace(config, "{{ KEY_PEM }}", strings.ReplaceAll(cm.keyPem, "\n", "\\n"), 1)
	//config = strings.Replace(config, "{{ CA_PEM }}", cm.caPem, 1)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: map[string]string{"config.yaml": config},
	}
}
