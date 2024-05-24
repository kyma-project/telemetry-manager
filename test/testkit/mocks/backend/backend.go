package backend

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/fluentd"
)

const (
	// telemetryDataFilename is the filename for the OpenTelemetry collector's file exporter.
	telemetryDataFilename = "otlp-data.jsonl"
	defaultNamespaceName  = "default"

	otlpGRPCPortName   = "grpc-otlp"
	otlpHTTPPortName   = "http-otlp"
	httpLogsPortName   = "http-logs"
	httpExportPortName = "http-web"

	otlpGRPCPort   = 4317
	otlpHTTPPort   = 4318
	httpLogsPort   = 9880
	httpExportPort = 80
)

const (
	DefaultName = "backend"
)

type SignalType string

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

	otelCollectorConfigMap      *ConfigMap
	fluentDConfigMap            *fluentd.ConfigMap
	otelCollectorDeployment     *Deployment
	otlpService                 *kitk8s.Service
	hostSecret                  *kitk8s.Secret
	virtualService              *kitk8s.VirtualService
	faultDelayPercentage        float64
	faultDelayFixedDelaySeconds int
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

func WithFaultDelayInjection(faultPercentage float64, delaySeconds int) Option {
	return func(b *Backend) {
		b.faultDelayPercentage = faultPercentage
		b.faultDelayFixedDelaySeconds = delaySeconds
	}
}

func (b *Backend) buildResources() {
	exportedFilePath := fmt.Sprintf("/%s/%s", string(b.signalType), telemetryDataFilename)

	b.otelCollectorConfigMap = NewConfigMap(fmt.Sprintf("%s-receiver-config", b.name), b.namespace, exportedFilePath, b.signalType, b.certs)
	b.otelCollectorDeployment = NewDeployment(b.name, b.namespace, b.otelCollectorConfigMap.Name(), filepath.Dir(exportedFilePath), b.replicas, b.signalType).WithAnnotations(map[string]string{"traffic.sidecar.istio.io/excludeInboundPorts": strconv.Itoa(httpExportPort)})
	b.otlpService = kitk8s.NewService(b.name, b.namespace).
		WithPort(otlpGRPCPortName, otlpGRPCPort).
		WithPort(otlpHTTPPortName, otlpHTTPPort).
		WithPort(httpExportPortName, httpExportPort)
	// TODO: LogPipelines requires the host and the port to be separated.
	// TracePipeline/MetricPipeline requires an endpoint in the format of scheme://host:port.
	// The referencable secret is called host in both cases, but the value is different. It has to be refactored.
	host := b.Endpoint()

	if b.signalType == SignalTypeLogs {
		b.fluentDConfigMap = fluentd.NewConfigMap(fmt.Sprintf("%s-receiver-config-fluentd", b.name), b.namespace, b.certs)
		b.otelCollectorDeployment.WithFluentdConfigName(b.fluentDConfigMap.Name())
		b.otlpService = b.otlpService.WithPort(httpLogsPortName, httpLogsPort)
		host = b.Host()
	}

	b.hostSecret = kitk8s.NewOpaqueSecret(fmt.Sprintf("%s-receiver-hostname", b.name), defaultNamespaceName,
		kitk8s.WithStringData("host", host)).Persistent(b.persistentHostSecret)

	if b.abortFaultPercentage > 0 || b.faultDelayPercentage > 0 {
		b.virtualService = kitk8s.NewVirtualService("fault-injection", b.namespace, b.name).WithFaultAbortPercentage(b.abortFaultPercentage).WithFaultDelay(b.faultDelayPercentage, time.Duration(b.faultDelayFixedDelaySeconds)*time.Second)
	}
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) Endpoint() string {
	addr := net.JoinHostPort(b.Host(), strconv.Itoa(b.Port()))
	return fmt.Sprintf("http://%s", addr)
}

func (b *Backend) Host() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", b.name, b.namespace)
}

func (b *Backend) Port() int {
	if b.signalType == SignalTypeLogs {
		return httpLogsPort
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
	return proxyClient.ProxyURLForService(b.namespace, b.name, telemetryDataFilename, httpExportPort)
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
	objects = append(objects, b.otelCollectorDeployment.K8sObject(kitk8s.WithLabel("app", b.name)))
	objects = append(objects, b.otlpService.K8sObject(kitk8s.WithLabel("app", b.name)))
	objects = append(objects, b.hostSecret.K8sObject())
	return objects
}
