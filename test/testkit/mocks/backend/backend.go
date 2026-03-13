package backend

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
)

const (
	otlpGRPCPortName = "grpc-otlp"
	otlpHTTPPortName = "http-otlp"
	httpLogsPortName = "http-logs"
	queryPortName    = "http-query"

	// Ports for pushing telemetry data to the backend (OTLP or FluentBit HTTP)
	otlpGRPCPort          int32 = 4317
	otlpHTTPPort          int32 = 4318
	httpFluentBitPushPort int32 = 9880
)

const (
	DefaultName       = "backend"
	QueryPath         = "otlp-data.jsonl"
	QueryPort   int32 = 80
)

type SignalType string

const (
	SignalTypeTraces        = "traces"
	SignalTypeMetrics       = "metrics"
	SignalTypeLogsFluentBit = "logs"
	SignalTypeLogsOTel      = "logs-otel"
	SignalTypeMetricsAgent  = "metrics"
)

type OIDCConfig struct {
	issuerURL string
	audience  string
}

type Backend struct {
	abortFaultPercentage float64
	dropFromSourceLabel  map[string]string
	certs                *testutils.ServerCerts
	name                 string
	namespace            string
	replicas             int32
	signalType           SignalType
	oidc                 *OIDCConfig
	mtls                 bool

	fluentDConfigMap    *fluentdConfigMapBuilder
	hostSecret          *kitk8sobjects.Secret
	collectorConfigMap  *collectorConfigMapBuilder
	collectorDeployment *collectorDeploymentBuilder
	collectorService    *kitk8sobjects.Service
	virtualService      *kitk8sobjects.VirtualService
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

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) Namespace() string {
	return b.namespace
}

func (b *Backend) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: b.name, Namespace: b.namespace}
}

func (b *Backend) EndpointHTTP() string {
	addr := net.JoinHostPort(b.Host(), strconv.Itoa(int(b.Port())))
	return fmt.Sprintf("http://%s", addr)
}

func (b *Backend) EndpointHTTPS() string {
	addr := net.JoinHostPort(b.Host(), strconv.Itoa(int(b.Port())))
	return fmt.Sprintf("https://%s", addr)
}

func (b *Backend) EndpointNoScheme() string {
	return net.JoinHostPort(b.Host(), strconv.Itoa(int(b.Port())))
}

func (b *Backend) Host() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", b.name, b.namespace)
}

func (b *Backend) Port() int32 {
	if b.signalType == SignalTypeLogsFluentBit {
		return httpFluentBitPushPort
	} else {
		return otlpGRPCPort
	}
}

func (b *Backend) HostSecretRefV1Alpha1() *telemetryv1alpha1.SecretKeyRef {
	return b.hostSecret.SecretKeyRefV1Alpha1("host")
}

func (b *Backend) HostSecretRefV1Beta1() *telemetryv1beta1.SecretKeyRef {
	return b.hostSecret.SecretKeyRefV1Beta1("host")
}

func (b *Backend) ExportURL(proxyClient *apiserverproxy.Client) string {
	return proxyClient.ProxyURLForService(b.namespace, b.name, QueryPath, QueryPort)
}

func (b *Backend) K8sObjects() []client.Object {
	var objects []client.Object
	if b.signalType == SignalTypeLogsFluentBit {
		// If FluentBit is used, a FluentD sidecar is added to the collector deployment.
		// The sidecar is connfigured to accept logs from FluentBit and forward them to the collector usngg the fluent protocol.
		// The data is then converted to OTLP and can be queried as usual.
		objects = append(objects, b.fluentDConfigMap.K8sObject())
	}

	if b.virtualService != nil {
		objects = append(objects, b.virtualService.K8sObject())
	}

	objects = append(objects, b.collectorConfigMap.K8sObject())
	objects = append(objects, b.collectorDeployment.K8sObject(kitk8sobjects.WithLabel("app", b.name)))
	objects = append(objects, b.collectorService.K8sObject(kitk8sobjects.WithLabel("app", b.name)))
	objects = append(objects, b.hostSecret.K8sObject())

	return objects
}

func (b *Backend) buildResources() {
	exportedFilePath := fmt.Sprintf("/%s/%s", string(b.signalType), QueryPath)

	b.collectorConfigMap = newCollectorConfigMap(
		fmt.Sprintf("%s-receiver-config", b.name),
		b.namespace,
		exportedFilePath,
		b.signalType,
		b.certs,
		b.oidc,
		b.mtls,
	)

	b.collectorDeployment = newCollectorDeployment(
		b.name,
		b.namespace,
		b.collectorConfigMap.Name(),
		filepath.Dir(exportedFilePath),
		b.replicas,
		b.signalType,
	).WithAnnotations(map[string]string{
		"traffic.sidecar.istio.io/excludeInboundPorts": strconv.Itoa(int(QueryPort)),
	})

	b.collectorService = kitk8sobjects.NewService(b.name, b.namespace).
		WithPort(otlpGRPCPortName, otlpGRPCPort).
		WithPort(otlpHTTPPortName, otlpHTTPPort).
		WithPort(queryPortName, QueryPort)

	// TODO: LogPipelines requires the host and the port to be separated.
	// TracePipeline/MetricPipeline requires an endpoint in the format of scheme://host:port.
	// The referencable secret is called host in both cases, but the value is different. It has to be refactored.
	host := b.EndpointHTTP()

	if b.signalType == SignalTypeLogsFluentBit {
		b.fluentDConfigMap = newFluentDConfigMapBuilder(
			fmt.Sprintf("%s-receiver-config-fluentd", b.name),
			b.namespace,
			b.certs,
		)
		b.collectorDeployment.WithFluentdConfigName(b.fluentDConfigMap.Name())
		b.collectorService = b.collectorService.WithPort(httpLogsPortName, httpFluentBitPushPort)
		host = b.Host()
	}

	b.hostSecret = kitk8sobjects.NewOpaqueSecret(
		fmt.Sprintf("%s-receiver-hostname", b.name),
		b.namespace,
		kitk8sobjects.WithStringData("host", host),
	)

	if b.abortFaultPercentage > 0 {
		// Configure fault injection for self-monitoring negative tests.
		b.virtualService = kitk8sobjects.NewVirtualService(
			"fault-injection",
			b.namespace,
			b.name,
		).WithFaultAbortPercentage(b.abortFaultPercentage).WithSourceLabel(b.dropFromSourceLabel)
	}
}
