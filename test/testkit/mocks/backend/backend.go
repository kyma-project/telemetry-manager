package backend

import (
	"fmt"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/fluentd"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

type SignalType string

const (
	// TelemetryDataFilename is the filename for the OpenTelemetry collector's file exporter.
	TelemetryDataFilename = "otlp-data.jsonl"
	defaultNamespaceName  = "default"

	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
	SignalTypeLogs    = "logs"
)

type Option func(*Backend)

type Backend struct {
	name       string
	namespace  string
	signalType SignalType

	persistent bool
	withTLS    bool

	configMap        *ConfigMap
	fluentDConfigMap *fluentd.ConfigMap
	deployment       *Deployment
	service          *kitk8s.Service
	hostSecret       *kitk8s.Secret

	TLSCerts tls.Certs
	Host     string
}

func New(name, namespace string, signalType SignalType, opts ...Option) *Backend {
	backend := &Backend{
		name:       name,
		namespace:  namespace,
		signalType: signalType,
	}

	for _, opt := range opts {
		opt(backend)
	}

	backend.buildResources()

	return backend
}

func WithTLS() Option {
	return func(b *Backend) {
		b.withTLS = true
	}
}

func Persistent(persistent bool) Option {
	return func(b *Backend) {
		b.persistent = persistent
	}
}

func (b *Backend) buildResources() {
	if b.withTLS {
		backendDNSName := fmt.Sprintf("%s.%s.svc.cluster.local", b.name, b.namespace)
		certs, err := tls.GenerateTLSCerts(backendDNSName)
		if err != nil {
			panic(fmt.Errorf("could not generate TLS certs: %v", err))
		}
		b.TLSCerts = certs
	}

	exportedFilePath := fmt.Sprintf("/%s/%s", string(b.signalType), TelemetryDataFilename)

	b.configMap = NewConfigMap(fmt.Sprintf("%s-receiver-config", b.name), b.namespace, exportedFilePath, b.signalType, b.withTLS, b.TLSCerts).Persistent(b.persistent)
	b.deployment = NewDeployment(b.name, b.namespace, b.configMap.Name(), filepath.Dir(exportedFilePath), b.signalType).Persistent(b.persistent)
	if b.signalType == SignalTypeLogs {
		b.fluentDConfigMap = fluentd.NewConfigMap(fmt.Sprintf("%s-receiver-config-fluentd", b.name), b.namespace, b.withTLS, b.TLSCerts).Persistent(b.persistent)
		b.deployment.WithFluentdConfigName(b.fluentDConfigMap.Name())
	}

	if b.signalType == SignalTypeLogs {
		b.service = kitk8s.NewService(b.name, b.namespace).
			WithPort(OTLPGRPCServiceName, ports.OTLPGRPC).
			WithPort(OTLPHTTPServiceName, ports.OTLPHTTP).
			WithPort(HTTPWebServiceName, HTTPWebPort).
			WithPort(HTTPLogServiceName, HTTPLogPort).
			Persistent(b.persistent)
		b.Host = fmt.Sprintf("%s.%s.svc.cluster.local", b.name, b.namespace)
	} else {
		b.service = kitk8s.NewService(b.name, b.namespace).
			WithPort(OTLPGRPCServiceName, ports.OTLPGRPC).
			WithPort(OTLPHTTPServiceName, ports.OTLPHTTP).
			WithPort(HTTPWebServiceName, HTTPWebPort).
			Persistent(b.persistent)
		b.Host = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", b.name, b.namespace, ports.OTLPGRPC)
	}

	b.hostSecret = kitk8s.NewOpaqueSecret(fmt.Sprintf("%s-receiver-hostname", b.name), defaultNamespaceName,
		kitk8s.WithStringData("host", b.Host)).
		Persistent(b.persistent)
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) HostSecretRef() *telemetryv1alpha1.SecretKeyRef {
	return b.hostSecret.SecretKeyRef("host")
}

func (b *Backend) TelemetryExportURL(proxyClient *apiserver.ProxyClient) string {
	return proxyClient.ProxyURLForService(b.namespace, b.name, TelemetryDataFilename, HTTPWebPort)
}

func (b *Backend) K8sObjects() []client.Object {
	var objects []client.Object
	if b.signalType == SignalTypeLogs {
		objects = append(objects, b.fluentDConfigMap.K8sObject())
	}

	objects = append(objects, b.configMap.K8sObject())
	objects = append(objects, b.deployment.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.service.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.hostSecret.K8sObject())
	return objects
}
