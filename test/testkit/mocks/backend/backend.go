package backend

import (
	"fmt"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
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

type Backend struct {
	name       string
	signalType SignalType

	ConfigMap        *ConfigMap
	FluentDConfigMap *fluentd.ConfigMap
	Deployment       *Deployment
	ExternalService  *ExternalService
	HostSecret       *kitk8s.Secret
}

func New(namespace string, o *Options) *Backend {
	b := &Backend{
		name:       o.Name,
		signalType: o.SignalType,
	}

	exportedFilePath := fmt.Sprintf("/%s/%s", string(o.SignalType), TelemetryDataFilename)

	b.ConfigMap = NewConfigMap(fmt.Sprintf("%s-receiver-config", b.name), namespace, exportedFilePath, b.signalType, o.WithTLS, o.TLSCerts)
	b.Deployment = NewDeployment(b.name, namespace, b.ConfigMap.Name(), filepath.Dir(exportedFilePath), b.signalType)
	b.ExternalService = NewExternalService(b.name, namespace, b.signalType)

	var endpoint string
	if b.signalType == SignalTypeLogs {
		endpoint = b.ExternalService.Host()
	} else {
		endpoint = b.ExternalService.OTLPGrpcEndpointURL()
	}
	b.HostSecret = kitk8s.NewOpaqueSecret(fmt.Sprintf("%s-receiver-hostname", b.name), defaultNamespaceName,
		kitk8s.WithStringData("host", endpoint)).Persistent(o.WithPersistentHostSecret)

	if b.signalType == SignalTypeLogs {
		b.FluentDConfigMap = fluentd.NewConfigMap(fmt.Sprintf("%s-receiver-config-fluentd", b.name), namespace)
	}

	return b
}

func (b *Backend) Name() string {
	return b.name
}

func (b *Backend) GetHostSecretRefKey() *telemetryv1alpha1.SecretKeyRef {
	return b.HostSecret.SecretKeyRef("host")
}

func (b *Backend) K8sObjects() []client.Object {
	var objects []client.Object
	objects = append(objects, b.ConfigMap.K8sObject())
	if b.signalType == SignalTypeLogs {
		objects = append(objects, b.FluentDConfigMap.K8sObject())
	}

	objects = append(objects, b.Deployment.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.ExternalService.K8sObject(kitk8s.WithLabel("app", b.Name())))
	objects = append(objects, b.HostSecret.K8sObject())
	return objects
}
