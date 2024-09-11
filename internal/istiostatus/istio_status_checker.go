package istiostatus

import (
	"context"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Checker struct {
	client client.Reader
}

const peerAuthenticationIstioCRD = "peerauthentications.security.istio.io"

func NewChecker(client client.Reader) *Checker {
	return &Checker{
		client: client,
	}

}

// IsIstioActive checks if Istio is active on the cluster based on the presence of Istio CRDs
func (isc *Checker) IsIstioActive(ctx context.Context) bool {
	var crdList apiextensionsv1.CustomResourceDefinitionList
	//add fieldselector to improve performance with large crds -> doesn't work with fakeclient
	//listOptions := &client.ListOptions{
	//	FieldSelector: fields.SelectorFromSet(fields.Set{"metadata.name": peerAuthenticationIstioCRD}),
	//}

	//add labelselector to improve performance with large crds by filtering to list only crds with certain labels
	//use labelselector because the fieldselector doesn't work with the fake client
	istioLabels := labels.Set{"app": "istio-pilot"}
	listOptions := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(istioLabels),
	}
	if err := isc.client.List(ctx, &crdList, listOptions); err != nil {
		logf.FromContext(ctx).Error(err, "Unable to list CRDs to check Istio status")
		return false
	}

	for _, crd := range crdList.Items {
		if crd.GetName() == peerAuthenticationIstioCRD {
			return true
		}
	}
	return false
}
