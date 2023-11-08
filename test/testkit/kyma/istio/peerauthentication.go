package istio

import (
	"istio.io/api/security/v1beta1"
	istiotypes "istio.io/api/type/v1beta1"
	securityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

type PeerAuthentication struct {
	name      string
	namespace string
}

func NewPeerAuthentication(name, namespace string) *PeerAuthentication {
	return &PeerAuthentication{
		name:      name,
		namespace: namespace,
	}
}

func (d *PeerAuthentication) K8sObject(labelOpts ...testkit.OptFunc) *securityv1beta1.PeerAuthentication {
	labels := k8s.ProcessLabelOptions(labelOpts...)
	workLoadSelector := istiotypes.WorkloadSelector{MatchLabels: labels}
	return &securityv1beta1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{Name: d.name, Namespace: d.namespace},
		Spec: v1beta1.PeerAuthentication{
			Selector: &workLoadSelector,
			Mtls:     &v1beta1.PeerAuthentication_MutualTLS{Mode: v1beta1.PeerAuthentication_MutualTLS_STRICT},
		},
	}
}
