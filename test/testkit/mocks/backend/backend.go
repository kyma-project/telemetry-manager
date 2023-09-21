package backend

import (
	"fmt"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/fluentd"
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

type Setter func(*Backend)

type Backend struct {
	name       string
	namespace  string
	signalType SignalType

	PersistentHostSecret bool
	WithTLS              bool
	TLSCerts             testkit.TLSCerts

	TracePipelineOptions  []kittrace.PipelineOption
	MetricPipelineOptions []kitmetric.PipelineOption

	ConfigMap        *ConfigMap
	FluentDConfigMap *fluentd.ConfigMap
	Deployment       *Deployment
	ExternalService  *ExternalService
	HostSecret       *kitk8s.Secret
}

func New(name, namespace string, signalType SignalType, setters ...Setter) (*Backend, error) {
	backend := &Backend{
		name:       name,
		namespace:  namespace,
		signalType: signalType,
	}

	for _, setter := range setters {
		setter(backend)
	}

	err := backend.build()
	return backend, err
}

func WithTLS() Setter {
	return func(b *Backend) {
		b.WithTLS = true
	}
}

func WithMetricPipelineOption(option kitmetric.PipelineOption) Setter {
	return func(b *Backend) {
		b.MetricPipelineOptions = append(b.MetricPipelineOptions, option)
	}
}

func WithPersistentHostSecret(persistentHostSecret bool) Setter {
	return func(b *Backend) {
		b.PersistentHostSecret = persistentHostSecret
	}
}

func (b *Backend) build() error {
	if b.WithTLS {
		backendDNSName := fmt.Sprintf("%s.%s.svc.cluster.local", b.name, b.namespace)
		certs, err := testkit.GenerateTLSCerts(backendDNSName)
		if err != nil {
			return err
		}
		b.TLSCerts = certs

		if b.signalType == SignalTypeMetrics {
			b.MetricPipelineOptions = append(b.MetricPipelineOptions, getTLSConfigMetricPipelineOption(
				certs.CaCertPem.String(), certs.ClientCertPem.String(), certs.ClientKeyPem.String()),
			)
		} else {
			b.TracePipelineOptions = append(b.TracePipelineOptions, getTLSConfigTracePipelineOption(
				certs.CaCertPem.String(), certs.ClientCertPem.String(), certs.ClientKeyPem.String()),
			)
		}
	}

	exportedFilePath := fmt.Sprintf("/%s/%s", string(b.signalType), TelemetryDataFilename)

	b.ConfigMap = NewConfigMap(fmt.Sprintf("%s-receiver-config", b.name), b.namespace, exportedFilePath, b.signalType, b.WithTLS, b.TLSCerts)
	b.Deployment = NewDeployment(b.name, b.namespace, b.ConfigMap.Name(), filepath.Dir(exportedFilePath), b.signalType)
	if b.signalType == SignalTypeLogs {
		b.FluentDConfigMap = fluentd.NewConfigMap(fmt.Sprintf("%s-receiver-config-fluentd", b.name), b.namespace)
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
		kitk8s.WithStringData("host", endpoint)).Persistent(b.PersistentHostSecret)

	return nil
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) HostSecretRefKey() *telemetryv1alpha1.SecretKeyRef {
	return b.HostSecret.SecretKeyRef("host")
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

func getTLSConfigMetricPipelineOption(caCertPem, clientCertPem, clientKeyPem string) kitmetric.PipelineOption {
	return func(metricPipeline telemetryv1alpha1.MetricPipeline) {
		metricPipeline.Spec.Output.Otlp.TLS = &telemetryv1alpha1.OtlpTLS{
			Insecure:           false,
			InsecureSkipVerify: false,
			CA: telemetryv1alpha1.ValueType{
				Value: caCertPem,
			},
			Cert: telemetryv1alpha1.ValueType{
				Value: clientCertPem,
			},
			Key: telemetryv1alpha1.ValueType{
				Value: clientKeyPem,
			},
		}
	}
}

func getTLSConfigTracePipelineOption(caCertPem, clientCertPem, clientKeyPem string) kittrace.PipelineOption {
	return func(tracePipeline telemetryv1alpha1.TracePipeline) {
		tracePipeline.Spec.Output.Otlp.TLS = &telemetryv1alpha1.OtlpTLS{
			Insecure:           false,
			InsecureSkipVerify: false,
			CA: telemetryv1alpha1.ValueType{
				Value: caCertPem,
			},
			Cert: telemetryv1alpha1.ValueType{
				Value: clientCertPem,
			},
			Key: telemetryv1alpha1.ValueType{
				Value: clientKeyPem,
			},
		}
	}
}
