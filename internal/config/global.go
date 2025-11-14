package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

// ValidationError represents a validation error for a specific field
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s' with value '%s': %s", e.Field, e.Value, e.Message)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

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

func (g *Global) Validate() error {
	if !namespaces.ValidNameRegexp.MatchString(g.namespace) {
		return &ValidationError{
			Field:   "namespace",
			Value:   g.namespace,
			Message: "must be a valid Kubernetes namespace name",
		}
	}

	if strings.Trim(g.version, " ") == "" {
		return &ValidationError{
			Field:   "version",
			Value:   g.version,
			Message: "cannot be empty or whitespace only",
		}
	}

	return nil
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
