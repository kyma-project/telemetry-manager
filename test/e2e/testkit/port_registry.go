//go:build e2e

package testkit

type (
	PortMapping struct {
		ServicePort int32
		NodePort    int32
		HostPort    int32
	}

	portMap map[string]PortMapping

	PortRegistry struct {
		Ports portMap
	}
)

// NewPortRegistry returns an instance of a port registry.
func NewPortRegistry() PortRegistry {
	return PortRegistry{
		Ports: make(portMap),
	}
}

// ServicePort returns a registered port value by its name.
func (p PortRegistry) ServicePort(name string) int32 {
	if _, ok := p.Ports[name]; !ok {
		return 0
	}

	return p.Ports[name].ServicePort
}

// NodePort returns a registered NodePort value by its name.
func (p PortRegistry) NodePort(name string) int32 {
	return p.Ports[name].NodePort
}

// HostPort returns a registered HostPort value by its name.
func (p PortRegistry) HostPort(name string) int32 {
	return p.Ports[name].HostPort
}

// AddServicePort adds a port mapping to the registry.
func (p PortRegistry) AddServicePort(name string, servicePort int32) PortRegistry {
	p.Ports[name] = PortMapping{ServicePort: servicePort}

	return p
}

// AddPortMapping adds a port mapping to the registry.
func (p PortRegistry) AddPortMapping(name string, servicePort, nodePort, hostPort int32) PortRegistry {
	p.Ports[name] = PortMapping{
		ServicePort: servicePort,
		NodePort:    nodePort,
		HostPort:    hostPort,
	}

	return p
}
