package istio

import (
	"context"
	"slices"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type StatusChecker struct {
	Client client.Reader
}

const peerAuthenticationIstioCRD = "peerauthentications.security.istio.io"

// IsIstioActive checks if Istio is active on the cluster based on the presence of Istio CRDs
func (isc *StatusChecker) IsIstioActive(ctx context.Context) bool {
	var crdList apiextensionsv1.CustomResourceDefinitionList
	if err := isc.Client.List(ctx, &crdList); err != nil {
		logf.FromContext(ctx).Error(err, "Unable to list CRDs to check Istio status")

		return false
	}

	return slices.ContainsFunc(crdList.Items, func(crd apiextensionsv1.CustomResourceDefinition) bool {
		return strings.EqualFold(crd.GetName(), peerAuthenticationIstioCRD)
	})
}
