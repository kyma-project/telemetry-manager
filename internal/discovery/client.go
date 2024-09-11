package discovery

import "k8s.io/client-go/discovery"

func MakeDiscoveryClient(apiConf APIConfig) (discovery.DiscoveryInterface, error) {
	if err := apiConf.Validate(); err != nil {
		return nil, err
	}

	authConf, err := CreateRestConfig(apiConf)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(authConf)
	if err != nil {
		return nil, err
	}

	return discoveryClient, nil
}
