//go:build e2e

package testkit

type (
	PortMapping struct {
		Port     int32
		NodePort int32
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

// Port returns a registered port value by its name.
func (p PortRegistry) Port(name string) int32 {
	if _, ok := p.Ports[name]; !ok {
		return 0
	}

	return p.Ports[name].Port
}

// NodePort returns a registered NodePort value by its name.
func (p PortRegistry) NodePort(name string) int32 {
	return p.Ports[name].NodePort
}

// AddPort adds a port mapping to the registry.
func (p PortRegistry) AddPort(name string, port int32) PortRegistry {
	p.Ports[name] = PortMapping{Port: port}

	return p
}

// AddPortMapping adds a port mapping to the registry.
func (p PortRegistry) AddPortMapping(name string, port, nodePort int32) PortRegistry {
	p.Ports[name] = PortMapping{
		Port:     port,
		NodePort: nodePort,
	}

	return p
}
