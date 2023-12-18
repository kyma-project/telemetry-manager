package istio

import (
	istiosecv1beta1 "istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiogosecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
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

func (d *PeerAuthentication) K8sObject(labelOpts ...testkit.OptFunc) *istiogosecv1beta1.PeerAuthentication {
	labels := kitk8s.ProcessLabelOptions(labelOpts...)
	workLoadSelector := istiotypev1beta1.WorkloadSelector{MatchLabels: labels}
	return &istiogosecv1beta1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{Name: d.name, Namespace: d.namespace},
		Spec: istiosecv1beta1.PeerAuthentication{
			Selector: &workLoadSelector,
			Mtls:     &istiosecv1beta1.PeerAuthentication_MutualTLS{Mode: istiosecv1beta1.PeerAuthentication_MutualTLS_STRICT},
		},
	}
}
