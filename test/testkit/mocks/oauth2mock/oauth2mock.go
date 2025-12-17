package oauth2mock

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
)

const (
	oauth2MockImage = "europe-central2-docker.pkg.dev/sap-se-cx-kyma-goat/networking-dev-tools/oauth2-mock:latest"

	DefaultName = "oauth2-mock"

	AudienceDefault = "default"
)

type OAuth2Authenticator struct {
	name                    string
	namespace               string
	authenticatorDeployment *kitk8sobjects.Deployment
	authenticatorService    *kitk8sobjects.Service
}

func New(namespace string) *OAuth2Authenticator {
	auth := &OAuth2Authenticator{
		name:      DefaultName,
		namespace: namespace,
	}

	auth.buildResources()

	return auth
}

func (o *OAuth2Authenticator) Name() string {
	return o.name
}

func (o *OAuth2Authenticator) Namespace() string {
	return o.namespace
}

func (o *OAuth2Authenticator) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: o.name, Namespace: o.namespace}
}

func (o *OAuth2Authenticator) Audience() string {
	return AudienceDefault
}

func (o *OAuth2Authenticator) TokenEndpoint() string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:8080/oauth2/token", o.name, o.namespace)
}

func (o *OAuth2Authenticator) IssuerURL() string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", o.name, o.namespace)
}

func (o *OAuth2Authenticator) K8sObjects() []client.Object {
	var objects []client.Object

	objects = append(objects, o.authenticatorDeployment.K8sObject())
	objects = append(objects, o.authenticatorService.K8sObject(kitk8sobjects.WithLabel("app", o.name)))

	return objects
}

func (o *OAuth2Authenticator) buildResources() {
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "oauth2-mock",
				Image: oauth2MockImage,
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 8080,
						Name:          "http",
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "iss",
						Value: o.IssuerURL(),
					},
				},
			},
		},
	}
	o.authenticatorDeployment = kitk8sobjects.NewDeployment(
		o.name,
		o.namespace,
	).WithReplicas(1).WithPodSpec(podSpec).WithLabel("app", o.name)

	o.authenticatorService = kitk8sobjects.NewService(
		o.name,
		o.namespace,
	).WithPort("http", 8080)
}
