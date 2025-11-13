package config

type Global struct {
	namespace      string
	enableFIPSMode bool
	version        string
}

type Option func(*Global)

func WithNamespace(namespace string) Option {
	return func(g *Global) {
		g.namespace = namespace
	}
}

func WithEnableFIPSMode(enableFIPSMode bool) Option {
	return func(g *Global) {
		g.enableFIPSMode = enableFIPSMode
	}
}

func WithVersion(version string) Option {
	return func(g *Global) {
		g.version = version
	}
}

func NewGlobal(opts ...Option) Global {
	g := Global{}
	for _, opt := range opts {
		opt(&g)
	}

	return g
}

func (g *Global) TargetNamespace() string {
	return g.namespace
}

func (g *Global) ManagerNamespace() string {
	return g.namespace
}

func (g *Global) DefaultTelemetryNamespace() string {
	return g.namespace
}

func (g *Global) EnableFIPSMode() bool {
	return g.enableFIPSMode
}

func (g *Global) Version() string {
	return g.version
}
