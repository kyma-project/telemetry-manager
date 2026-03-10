package istiostatus

import (
	"context"
	"strings"

	"k8s.io/client-go/discovery"
)

type Checker struct {
	discovery discovery.DiscoveryInterface
}

func NewChecker(d discovery.DiscoveryInterface) *Checker {
	return &Checker{discovery: d}
}

// IsIstioActive checks if Istio is active on the cluster based on the presence of Istio CRDs
func (isc *Checker) IsIstioActive(ctx context.Context) (bool, error) {
	groupList, err := isc.discovery.ServerGroups()
	if err != nil {
		return false, err
	}

	for _, group := range groupList.Groups {
		if strings.Contains(group.Name, ".istio.io") {
			return true, nil
		}
	}

	return false, nil
}
