package istiostatus

import (
	"context"
	"k8s.io/client-go/discovery"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

type Checker struct {
	discovery discovery.DiscoveryInterface
}

func NewChecker(d discovery.DiscoveryInterface) *Checker {
	return &Checker{discovery: d}
}

// IsIstioActive checks if Istio is active on the cluster based on the presence of Istio CRDs
func (isc *Checker) IsIstioActive(ctx context.Context) bool {

	groupList, err := isc.discovery.ServerGroups()
	if err != nil {
		logf.FromContext(ctx).Error(err, "error getting group list from server")
	}

	for _, group := range groupList.Groups {
		if strings.Contains(group.Name, ".istio.io") {
			return true
		}
	}
	return false

}
