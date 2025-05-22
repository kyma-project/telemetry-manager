package backend

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

type fluentdConfigMapBuilder struct {
	name      string
	namespace string
	certs     *testutils.ServerCerts
}

func newFluentDConfigMapBuilder(name, namespace string, certs *testutils.ServerCerts) *fluentdConfigMapBuilder {
	return &fluentdConfigMapBuilder{
		name:      name,
		namespace: namespace,
		certs:     certs,
	}
}

const configTemplateFluentd = `<system>
  log_level fatal
</system>
<source>
  @type http
  port 9880
  bind 0.0.0.0
  body_size_limit 32m
  add_http_headers true
  <parse>
    @type json
  </parse>
</source>
<match **>
  @type forward
  send_timeout 60s
  recover_wait 10s
  hard_timeout 60s
  flush_interval 1s

  <server>
    name otlp
    host 127.0.0.1
    port 8006
    weight 60
  </server>
</match>`

const configTemplateFluentdTLS = `<system>
  log_level fatal
</system>
<source>
  @type http
  port 9880
  bind 0.0.0.0
  body_size_limit 32m
  add_http_headers true
  <parse>
    @type json
  </parse>
  <transport tls>
    cert_path /fluentd/etc/server.crt
    private_key_path /fluentd/etc/server.key
    ca_path /fluentd/etc/ca.crt
    client_cert_auth true
  </transport>
</source>
<match **>
  @type forward
  send_timeout 60s
  recover_wait 10s
  hard_timeout 60s
  flush_interval 1s

  <server>
    name otlp
    host 127.0.0.1
    port 8006
    weight 60
  </server>
</match>`

func (cm *fluentdConfigMapBuilder) Name() string {
	return cm.name
}

func (cm *fluentdConfigMapBuilder) K8sObject() *corev1.ConfigMap {
	data := make(map[string]string)
	if cm.certs != nil {
		data["fluent.conf"] = configTemplateFluentdTLS
		data["server.crt"] = cm.certs.ServerCertPem.String()
		data["server.key"] = cm.certs.ServerKeyPem.String()
		data["ca.crt"] = cm.certs.CaCertPem.String()
	} else {
		data["fluent.conf"] = configTemplateFluentd
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: data,
	}
}
