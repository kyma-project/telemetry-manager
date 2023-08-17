package testkit

type (
	portMap map[string]int32

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

	return p.Ports[name]
}

// AddPort adds a port mapping to the registry.
func (p PortRegistry) AddPort(name string, port int) PortRegistry {
	p.Ports[name] = int32(port)

	return p
}
