package coordinationconfig

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

const (
	// ConfigMapDataKey is the key in the ConfigMap data that contains the pipeline references
	ConfigMapDataKey = "pipelines.yaml"
)

// OTLPGatewayConfigMap represents the structure of the OTLP Gateway coordination ConfigMap.
// It contains references to pipelines that should be included in the OTLP Gateway configuration.
type OTLPGatewayConfigMap struct {
	TracePipelineReferences  []PipelineReference `yaml:"tracePipelines,omitempty"`
	LogPipelineReferences    []PipelineReference `yaml:"logPipelines,omitempty"`
	MetricPipelineReferences []PipelineReference `yaml:"metricPipelines,omitempty"`
}

// PipelineReference contains minimal information about a pipeline.
// The controller must fetch the full pipeline spec using the name.
type PipelineReference struct {
	Name           string            `yaml:"name"`
	Generation     int64             `yaml:"generation"`
	SecretVersions map[string]string `yaml:"secretVersions,omitempty"`
}

// PipelineReferenceInput contains the data needed to write a pipeline reference.
type PipelineReferenceInput struct {
	Name           string
	Generation     int64
	SecretVersions map[string]string
}

// ReadOTLPGatewayConfig reads and parses the OTLP Gateway Coordination ConfigMap.
// Returns an empty configuration if the ConfigMap doesn't exist.
func ReadOTLPGatewayConfig(ctx context.Context, c client.Client, namespace string) (*OTLPGatewayConfigMap, error) {
	var cm corev1.ConfigMap

	err := c.Get(ctx, types.NamespacedName{
		Name:      names.OTLPGatewayCoordinationConfigMap,
		Namespace: namespace,
	}, &cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &OTLPGatewayConfigMap{}, nil
		}

		return nil, fmt.Errorf("failed to get OTLP Gateway coordination ConfigMap: %w", err)
	}

	yamlData, ok := cm.Data[ConfigMapDataKey]
	if !ok || yamlData == "" {
		return &OTLPGatewayConfigMap{}, nil
	}

	var config OTLPGatewayConfigMap
	if err := yaml.Unmarshal([]byte(yamlData), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ConfigMap data: %w", err)
	}

	return &config, nil
}

// CollectSecretVersions fetches the resourceVersion for each secret reference.
// Returns a map of "namespace/name" -> resourceVersion.
// Secrets that are not found (404) are skipped. Any other error is returned.
func CollectSecretVersions(ctx context.Context, c client.Client, refs []telemetryv1beta1.SecretKeyRef) (map[string]string, error) {
	versions := make(map[string]string)
	seen := make(map[types.NamespacedName]bool)

	for _, ref := range refs {
		key := types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}

		// Skip duplicates
		if seen[key] {
			continue
		}

		seen[key] = true

		var secret corev1.Secret
		if err := c.Get(ctx, key, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return nil, fmt.Errorf("failed to get secret %s/%s: %w", ref.Namespace, ref.Name, err)
		}

		mapKey := fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
		versions[mapKey] = secret.ResourceVersion
	}

	return versions, nil
}

// AddPipelineReference adds or updates a pipeline reference of any type.
// Uses optimistic locking to handle concurrent updates safely.
func AddPipelineReference(ctx context.Context, c client.Client, namespace string, pipelineType common.SignalType, input PipelineReferenceInput) error {
	return applyConfigUpdate(ctx, c, namespace, func(config *OTLPGatewayConfigMap) error {
		pipelineSlice := getPipelineSlice(config, pipelineType)
		if pipelineSlice == nil {
			return fmt.Errorf("invalid pipeline type: %s", pipelineType)
		}

		// Find existing reference
		for i := range *pipelineSlice {
			if (*pipelineSlice)[i].Name == input.Name {
				// Update generation and secret versions
				(*pipelineSlice)[i].Generation = input.Generation
				(*pipelineSlice)[i].SecretVersions = input.SecretVersions

				return nil
			}
		}

		// Add new reference
		*pipelineSlice = append(*pipelineSlice, PipelineReference(input))

		return nil
	})
}

// RemovePipelineReference removes a pipeline reference of any type.
// Uses optimistic locking to handle concurrent updates safely.
func RemovePipelineReference(ctx context.Context, c client.Client, namespace string, pipelineType common.SignalType, name string) error {
	return applyConfigUpdate(ctx, c, namespace, func(config *OTLPGatewayConfigMap) error {
		pipelineSlice := getPipelineSlice(config, pipelineType)
		if pipelineSlice == nil {
			return fmt.Errorf("invalid pipeline type: %s", pipelineType)
		}

		// Filter out the reference
		filtered := make([]PipelineReference, 0, len(*pipelineSlice))
		for _, ref := range *pipelineSlice {
			if ref.Name != name {
				filtered = append(filtered, ref)
			}
		}

		*pipelineSlice = filtered

		return nil
	})
}

// getPipelineSlice returns a pointer to the appropriate pipeline slice based on type.
func getPipelineSlice(config *OTLPGatewayConfigMap, pipelineType common.SignalType) *[]PipelineReference {
	switch pipelineType {
	case common.SignalTypeTrace:
		return &config.TracePipelineReferences
	case common.SignalTypeLog:
		return &config.LogPipelineReferences
	case common.SignalTypeMetric:
		return &config.MetricPipelineReferences
	default:
		return nil
	}
}

// applyConfigUpdate reads the coordination ConfigMap, applies updateFn to it, and writes it back.
// Errors are returned directly and propagated to the caller's reconciliation loop.
func applyConfigUpdate(ctx context.Context, c client.Client, namespace string, updateFn func(*OTLPGatewayConfigMap) error) error {
	cm, exists, err := getConfigMap(ctx, c, namespace)
	if err != nil {
		return err
	}

	config, err := parseConfig(cm, exists)
	if err != nil {
		return err
	}

	if err := updateFn(&config); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if !exists {
		return createConfigMap(ctx, c, namespace, string(yamlData))
	}

	return updateConfigMap(ctx, c, cm, string(yamlData))
}

// getConfigMap fetches the ConfigMap and returns whether it exists
func getConfigMap(ctx context.Context, c client.Client, namespace string) (*corev1.ConfigMap, bool, error) {
	var cm corev1.ConfigMap

	err := c.Get(ctx, types.NamespacedName{
		Name:      names.OTLPGatewayCoordinationConfigMap,
		Namespace: namespace,
	}, &cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	return &cm, true, nil
}

// parseConfig parses the configuration from a ConfigMap
func parseConfig(cm *corev1.ConfigMap, exists bool) (OTLPGatewayConfigMap, error) {
	var config OTLPGatewayConfigMap

	if !exists {
		return config, nil
	}

	yamlData, ok := cm.Data[ConfigMapDataKey]
	if !ok || yamlData == "" {
		return config, nil
	}

	if err := yaml.Unmarshal([]byte(yamlData), &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal ConfigMap: %w", err)
	}

	return config, nil
}

// createConfigMap creates a new ConfigMap with the given YAML data
func createConfigMap(ctx context.Context, c client.Client, namespace, yamlData string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.OTLPGatewayCoordinationConfigMap,
			Namespace: namespace,
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	if err := c.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	return nil
}

// updateConfigMap updates an existing ConfigMap with new YAML data
func updateConfigMap(ctx context.Context, c client.Client, cm *corev1.ConfigMap, yamlData string) error {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	cm.Data[ConfigMapDataKey] = yamlData

	if err := c.Update(ctx, cm); err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return nil
}
