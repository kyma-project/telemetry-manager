package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

const (
	errMsgInvalidNamespace  = "must be a valid Kubernetes namespace name"
	errMsgEmptyOrWhitespace = "cannot be empty or whitespace only"
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
	managerNamespace       string
	targetNamespace        string
	operateInFIPSMode      bool
	version                string
	imagePullSecretName    string
	clusterTrustBundleName string
	additionalLabels       map[string]string
	additionalAnnotations  map[string]string
}

type Option func(*Global)

func WithManagerNamespace(namespace string) Option {
	return func(g *Global) {
		g.managerNamespace = namespace
	}
}

func WithTargetNamespace(namespace string) Option {
	return func(g *Global) {
		g.targetNamespace = namespace
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

func WithImagePullSecretName(secretName string) Option {
	return func(g *Global) {
		g.imagePullSecretName = secretName
	}
}

func WithClusterTrustBundleName(name string) Option {
	return func(g *Global) {
		g.clusterTrustBundleName = name
	}
}

func WithAdditionalLabels(labels map[string]string) Option {
	return func(g *Global) {
		g.additionalLabels = labels
	}
}

func WithAdditionalAnnotations(annotations map[string]string) Option {
	return func(g *Global) {
		g.additionalAnnotations = annotations
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
	if !namespaces.ValidNameRegexp.MatchString(g.targetNamespace) {
		return &ValidationError{Field: "target_namespace", Value: g.targetNamespace, Message: errMsgInvalidNamespace}
	}

	if !namespaces.ValidNameRegexp.MatchString(g.managerNamespace) {
		return &ValidationError{Field: "manager_namespace", Value: g.managerNamespace, Message: errMsgInvalidNamespace}
	}

	if strings.Trim(g.version, " ") == "" {
		return &ValidationError{Field: "version", Value: g.version, Message: errMsgEmptyOrWhitespace}
	}

	return nil
}

// TargetNamespace returns the namespace where telemetry components should be deployed by Telemetry Manager.
func (g *Global) TargetNamespace() string {
	return g.targetNamespace
}

// ManagerNamespace returns the namespace where Telemetry Manager is deployed.
// In a Kyma setup, this is the same as TargetNamespace.
func (g *Global) ManagerNamespace() string {
	return g.managerNamespace
}

// DefaultTelemetryNamespace returns the namespace where default Telemetry CR (containing module config) is located.
// In a Kyma setup, this is the same as TargetNamespace.
func (g *Global) DefaultTelemetryNamespace() string {
	return g.targetNamespace
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

func (g *Global) ImagePullSecretName() string {
	return g.imagePullSecretName
}

func (g *Global) ClusterTrustBundleName() string {
	return g.clusterTrustBundleName
}

func (g *Global) AdditionalLabels() map[string]string {
	return g.additionalLabels
}

func (g *Global) AdditionalAnnotations() map[string]string {
	return g.additionalAnnotations
}
