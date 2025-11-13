package config

type Global struct {
	namespace         string
	operateInFIPSMode bool
	version           string
}

type Option func(*Global)

// WithNamespace sets both TargetNamespace, DefaultTelemetryNamespace and ManagerNamespace to the given value.
// TODO: Split into separate options.
func WithNamespace(namespace string) Option {
	return func(g *Global) {
		g.namespace = namespace
	}
}

func WithOperateInFIPSMode(enable bool) Option {
	return func(g *Global) {
		g.operateInFIPSMode = enable
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

// TargetNamespace returns the namespace where telemetry components should be deployed by Telemetry Manager.
func (g *Global) TargetNamespace() string {
	return g.namespace
}

// ManagerNamespace returns the namespace where Telemetry Manager is deployed.
// In a Kyma setup, this is the same as TargetNamespace.
func (g *Global) ManagerNamespace() string {
	return g.namespace
}

// DefaultTelemetryNamespace returns the namespace where default Telemetry CR (containing module config) is located.
// In a Kyma setup, this is the same as TargetNamespace.
func (g *Global) DefaultTelemetryNamespace() string {
	return g.namespace
}

// OperateInFIPSMode indicates whether telemetry components should operate in FIPS mode.
// Note, that it does not apply to the Telemetry Manager itself, which always runs in FIPS mode (see Dockerfile for details).
func (g *Global) OperateInFIPSMode() bool {
	return g.operateInFIPSMode
}

// Version returns the version of the Telemetry Manager.
func (g *Global) Version() string {
	return g.version
}
