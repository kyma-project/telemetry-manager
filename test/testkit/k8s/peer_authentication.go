package k8s

import (
	istiosecurityv1beta "istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecurityclientv1beta "istio.io/client-go/pkg/apis/security/v1beta1"
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

func (d *PeerAuthentication) K8sObject(labelOpts ...testkit.OptFunc) *istiosecurityclientv1beta.PeerAuthentication {
	labels := ProcessLabelOptions(labelOpts...)
	workLoadSelector := istiotypev1beta1.WorkloadSelector{MatchLabels: labels}
	return &istiosecurityclientv1beta.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{Name: d.name, Namespace: d.namespace},
		Spec: istiosecurityv1beta.PeerAuthentication{
			Selector: &workLoadSelector,
			Mtls:     &istiosecurityv1beta.PeerAuthentication_MutualTLS{Mode: istiosecurityv1beta.PeerAuthentication_MutualTLS_STRICT},
		},
	}
}
