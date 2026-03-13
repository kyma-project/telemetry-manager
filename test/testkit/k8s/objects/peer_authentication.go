package objects

import (
	istiosecurityv1 "istio.io/api/security/v1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/test/testkit"
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

func (d *PeerAuthentication) K8sObject(labelOpts ...testkit.OptFunc) *istiosecurityclientv1.PeerAuthentication {
	labels := ProcessLabelOptions(labelOpts...)
	workLoadSelector := istiotypev1beta1.WorkloadSelector{MatchLabels: labels}

	return &istiosecurityclientv1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{Name: d.name, Namespace: d.namespace},
		Spec: istiosecurityv1.PeerAuthentication{
			Selector: &workLoadSelector,
			Mtls:     &istiosecurityv1.PeerAuthentication_MutualTLS{Mode: istiosecurityv1.PeerAuthentication_MutualTLS_STRICT},
		},
	}
}
