package backend

import (
	"fmt"
	"path/filepath"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/fluentd"
	"github.com/kyma-project/telemetry-manager/test/testkit/tlsgen"
)

type SignalType string

const (
	// telemetryDataFilename is the filename for the OpenTelemetry collector's file exporter.
	telemetryDataFilename = "otlp-data.jsonl"
	defaultNamespaceName  = "default"
)

const (
	DefaultName = "backend"
)

const (
	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
	SignalTypeLogs    = "logs"
)

type Option func(*Backend)

type Backend struct {
	name       string
	namespace  string
	signalType SignalType

	persistentHostSecret bool
	certs                *tlsgen.ServerCerts

	ConfigMap        *ConfigMap
	FluentDConfigMap *fluentd.ConfigMap
	Deployment       *Deployment
	ExternalService  *ExternalService
	HostSecret       *kitk8s.Secret
}

func New(namespace string, signalType SignalType, opts ...Option) *Backend {
	backend := &Backend{
		name:       DefaultName,
		namespace:  namespace,
		signalType: signalType,
	}

	for _, opt := range opts {
		opt(backend)
	}

	backend.buildResources()

	return backend
}

func WithName(name string) Option {
	return func(b *Backend) {
		b.name = name
	}
}

func WithTLS(certKey tlsgen.ServerCerts) Option {
	return func(b *Backend) {
		b.certs = &certKey
	}
}

func WithPersistentHostSecret(persistentHostSecret bool) Option {
	return func(b *Backend) {
		b.persistentHostSecret = persistentHostSecret
	}
}

func (b *Backend) buildResources() {
	exportedFilePath := fmt.Sprintf("/%s/%s", string(b.signalType), telemetryDataFilename)

	b.ConfigMap = NewConfigMap(fmt.Sprintf("%s-receiver-config", b.name), b.namespace, exportedFilePath, b.signalType, b.certs)
	b.Deployment = NewDeployment(b.name, b.namespace, b.ConfigMap.Name(), filepath.Dir(exportedFilePath), b.signalType).WithAnnotations(map[string]string{"traffic.sidecar.istio.io/excludeInboundPorts": strconv.Itoa(HTTPWebPort)})

	if b.signalType == SignalTypeLogs {
		b.FluentDConfigMap = fluentd.NewConfigMap(fmt.Sprintf("%s-receiver-config-fluentd", b.name), b.namespace, b.certs)
		b.Deployment.WithFluentdConfigName(b.FluentDConfigMap.Name())
	}

	b.ExternalService = NewExternalService(b.name, b.namespace, b.signalType)

	var endpoint string
	if b.signalType == SignalTypeLogs {
		endpoint = b.ExternalService.Host()
	} else {
		endpoint = b.ExternalService.OTLPGrpcEndpointURL()
	}
	b.HostSecret = kitk8s.NewOpaqueSecret(fmt.Sprintf("%s-receiver-hostname", b.name), defaultNamespaceName,
		kitk8s.WithStringData("host", endpoint)).Persistent(b.persistentHostSecret)
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) HostSecretRefV1Alpha1() *telemetryv1alpha1.SecretKeyRef {
	return b.HostSecret.SecretKeyRefV1Alpha1("host")
}

func (b *Backend) HostSecretRefV1Beta1() *telemetryv1beta1.SecretKeyRef {
	return b.HostSecret.SecretKeyRefV1Beta1("host")
}

func (b *Backend) ExportURL(proxyClient *apiserverproxy.Client) string {
	return proxyClient.ProxyURLForService(b.namespace, b.name, telemetryDataFilename, HTTPWebPort)
}

func (b *Backend) K8sObjects() []client.Object {
	var objects []client.Object
	if b.signalType == SignalTypeLogs {
		objects = append(objects, b.FluentDConfigMap.K8sObject())
	}

	objects = append(objects, b.ConfigMap.K8sObject())
	objects = append(objects, b.Deployment.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.ExternalService.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.HostSecret.K8sObject())
	return objects
}
