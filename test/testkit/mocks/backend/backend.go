package backend

import (
	"fmt"
	"path/filepath"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/fluentd"
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
	name                 string
	namespace            string
	replicas             int32
	signalType           SignalType
	persistentHostSecret bool
	certs                *testutils.ServerCerts
	abortFaultPercentage float64

	otelCollectorConfigMap  *ConfigMap
	fluentDConfigMap        *fluentd.ConfigMap
	otelCollectorDeployment *Deployment
	hostSecret              *kitk8s.Secret
	virtualService          *kitk8s.VirtualService

	ExternalService *ExternalService
}

func New(namespace string, signalType SignalType, opts ...Option) *Backend {
	backend := &Backend{
		name:       DefaultName,
		namespace:  namespace,
		replicas:   1,
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

func WithReplicas(replicas int32) Option {
	return func(b *Backend) {
		b.replicas = replicas
	}
}

func WithTLS(certKey testutils.ServerCerts) Option {
	return func(b *Backend) {
		b.certs = &certKey
	}
}

func WithPersistentHostSecret(persistentHostSecret bool) Option {
	return func(b *Backend) {
		b.persistentHostSecret = persistentHostSecret
	}
}

func WithAbortFaultInjection(abortFaultPercentage float64) Option {
	return func(b *Backend) {
		b.abortFaultPercentage = abortFaultPercentage
	}
}

func (b *Backend) buildResources() {
	exportedFilePath := fmt.Sprintf("/%s/%s", string(b.signalType), telemetryDataFilename)

	b.otelCollectorConfigMap = NewConfigMap(fmt.Sprintf("%s-receiver-config", b.name), b.namespace, exportedFilePath, b.signalType, b.certs)
	b.otelCollectorDeployment = NewDeployment(b.name, b.namespace, b.otelCollectorConfigMap.Name(), filepath.Dir(exportedFilePath), b.replicas, b.signalType).WithAnnotations(map[string]string{"traffic.sidecar.istio.io/excludeInboundPorts": strconv.Itoa(HTTPWebPort)})

	if b.signalType == SignalTypeLogs {
		b.fluentDConfigMap = fluentd.NewConfigMap(fmt.Sprintf("%s-receiver-config-fluentd", b.name), b.namespace, b.certs)
		b.otelCollectorDeployment.WithFluentdConfigName(b.fluentDConfigMap.Name())
	}

	b.ExternalService = NewExternalService(b.name, b.namespace, b.signalType)

	b.hostSecret = kitk8s.NewOpaqueSecret(fmt.Sprintf("%s-receiver-hostname", b.name), defaultNamespaceName,
		kitk8s.WithStringData("host", b.Host())).Persistent(b.persistentHostSecret)

	if b.abortFaultPercentage > 0 {
		b.virtualService = kitk8s.NewVirtualService("fault-injection", b.namespace, b.name).WithAbortFaultPercentage(b.abortFaultPercentage)
	}
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) Host() string {
	if b.ExternalService == nil {
		return ""
	}

	if b.signalType == SignalTypeLogs {
		return b.ExternalService.Host()
	} else {
		return b.ExternalService.OTLPGrpcEndpointURL()
	}
}

func (b *Backend) Port() int {
	if b.signalType == SignalTypeLogs {
		return HTTPLogPort
	} else {
		return ports.OTLPGRPC
	}
}

func (b *Backend) HostSecretRefV1Alpha1() *telemetryv1alpha1.SecretKeyRef {
	return b.hostSecret.SecretKeyRefV1Alpha1("host")
}

func (b *Backend) HostSecretRefV1Beta1() *telemetryv1beta1.SecretKeyRef {
	return b.hostSecret.SecretKeyRefV1Beta1("host")
}

func (b *Backend) ExportURL(proxyClient *apiserverproxy.Client) string {
	return proxyClient.ProxyURLForService(b.namespace, b.name, telemetryDataFilename, HTTPWebPort)
}

func (b *Backend) K8sObjects() []client.Object {
	var objects []client.Object
	if b.signalType == SignalTypeLogs {
		objects = append(objects, b.fluentDConfigMap.K8sObject())
	}

	if b.virtualService != nil {
		objects = append(objects, b.virtualService.K8sObject())
	}

	objects = append(objects, b.otelCollectorConfigMap.K8sObject())
	objects = append(objects, b.otelCollectorDeployment.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.ExternalService.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.hostSecret.K8sObject())
	return objects
}
