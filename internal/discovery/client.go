package discovery

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

func MakeDiscoveryClient(restConfig *rest.Config) (discovery.DiscoveryInterface, error) {

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return discoveryClient, nil
}
