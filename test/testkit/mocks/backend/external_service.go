package backend

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

const (
	OTLPGRPCServiceName = "grpc-otlp"
	OTLPHTTPServiceName = "http-otlp"
	HTTPWebServiceName  = "http-web"
	HTTPLogServiceName  = "http-log"

	HTTPWebPort = 80
	HTTPLogPort = 9880
)

type ExternalService struct {
	*kitk8s.Service
	name      string
	namespace string
}

func NewExternalService(name, namespace string, signalType SignalType) *ExternalService {
	svc := kitk8s.NewService(name, namespace).
		WithPort(OTLPGRPCServiceName, ports.OTLPGRPC).
		WithPort(OTLPHTTPServiceName, ports.OTLPHTTP).
		WithPort(HTTPWebServiceName, HTTPWebPort)

	if signalType == SignalTypeLogs {
		svc = svc.WithPort(HTTPLogServiceName, HTTPLogPort)
	}

	return &ExternalService{Service: svc, name: name, namespace: namespace}
}

func (s *ExternalService) OTLPGrpcEndpointURL() string {
	port := s.Ports[OTLPGRPCServiceName]

	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", s.name, s.namespace, port)
}

func (s *ExternalService) Host() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", s.name, s.namespace)
}
