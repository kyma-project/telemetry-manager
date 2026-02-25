package otelcollector

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// OTLPGatewayConfigMapName is the name of the ConfigMap that coordinates pipeline information
	OTLPGatewayConfigMapName = "telemetry-otlp-gateway-config"

	// ConfigMapDataKey is the key in the ConfigMap data that contains the pipeline references
	ConfigMapDataKey = "pipelines.yaml"

	// maxRetries is the maximum number of retry attempts for ConfigMap updates
	maxRetries = 5
)

// OTLPGatewayConfigMap represents the structure of the OTLP Gateway coordination ConfigMap.
// It contains references to pipelines that should be included in the OTLP Gateway configuration.
type OTLPGatewayConfigMap struct {
	TracePipeline  []PipelineReference `yaml:"TracePipeline,omitempty"`
	LogPipeline    []PipelineReference `yaml:"LogPipeline,omitempty"`
	MetricPipeline []PipelineReference `yaml:"MetricPipeline,omitempty"`
}

// PipelineReference contains minimal information about a pipeline.
// The controller must fetch the full pipeline spec using the name.
type PipelineReference struct {
	Name       string `yaml:"name"`
	Generation int64  `yaml:"generation"`
}

// ReadOTLPGatewayConfig reads and parses the OTLP Gateway ConfigMap.
// Returns an empty configuration if the ConfigMap doesn't exist.
func ReadOTLPGatewayConfig(ctx context.Context, c client.Client, namespace string) (*OTLPGatewayConfigMap, error) {
	var cm corev1.ConfigMap

	err := c.Get(ctx, types.NamespacedName{
		Name:      OTLPGatewayConfigMapName,
		Namespace: namespace,
	}, &cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &OTLPGatewayConfigMap{}, nil
		}

		return nil, fmt.Errorf("failed to get otlp gateway configmap: %w", err)
	}

	yamlData, ok := cm.Data[ConfigMapDataKey]
	if !ok || yamlData == "" {
		return &OTLPGatewayConfigMap{}, nil
	}

	var config OTLPGatewayConfigMap
	if err := yaml.Unmarshal([]byte(yamlData), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap data: %w", err)
	}

	return &config, nil
}

// WriteTracePipelineReference adds or updates a TracePipeline reference.
// Uses optimistic locking with retry to handle concurrent updates safely.
func WriteTracePipelineReference(ctx context.Context, c client.Client, namespace, name string, generation int64) error {
	return updateConfigMapWithRetry(ctx, c, namespace, func(config *OTLPGatewayConfigMap) error {
		// Find existing reference
		for i := range config.TracePipeline {
			if config.TracePipeline[i].Name == name {
				// Update generation
				config.TracePipeline[i].Generation = generation
				return nil
			}
		}

		// Add new reference
		config.TracePipeline = append(config.TracePipeline, PipelineReference{
			Name:       name,
			Generation: generation,
		})

		return nil
	})
}

// RemoveTracePipelineReference removes a TracePipeline reference.
// Uses optimistic locking with retry. Idempotent operation.
func RemoveTracePipelineReference(ctx context.Context, c client.Client, namespace, name string) error {
	return updateConfigMapWithRetry(ctx, c, namespace, func(config *OTLPGatewayConfigMap) error {
		// Filter out the reference
		filtered := make([]PipelineReference, 0, len(config.TracePipeline))
		for _, ref := range config.TracePipeline {
			if ref.Name != name {
				filtered = append(filtered, ref)
			}
		}

		config.TracePipeline = filtered

		return nil
	})
}

// WriteLogPipelineReference adds or updates a LogPipeline reference.
// Uses optimistic locking with retry to handle concurrent updates safely.
func WriteLogPipelineReference(ctx context.Context, c client.Client, namespace, name string, generation int64) error {
	return updateConfigMapWithRetry(ctx, c, namespace, func(config *OTLPGatewayConfigMap) error {
		// Find existing reference
		for i := range config.LogPipeline {
			if config.LogPipeline[i].Name == name {
				// Update generation
				config.LogPipeline[i].Generation = generation
				return nil
			}
		}

		// Add new reference
		config.LogPipeline = append(config.LogPipeline, PipelineReference{
			Name:       name,
			Generation: generation,
		})

		return nil
	})
}

// RemoveLogPipelineReference removes a LogPipeline reference.
// Uses optimistic locking with retry. Idempotent operation.
func RemoveLogPipelineReference(ctx context.Context, c client.Client, namespace, name string) error {
	return updateConfigMapWithRetry(ctx, c, namespace, func(config *OTLPGatewayConfigMap) error {
		// Filter out the reference
		filtered := make([]PipelineReference, 0, len(config.LogPipeline))
		for _, ref := range config.LogPipeline {
			if ref.Name != name {
				filtered = append(filtered, ref)
			}
		}

		config.LogPipeline = filtered

		return nil
	})
}

// updateConfigMapWithRetry implements optimistic locking with retry.
// Retries on 409 Conflict errors with fresh data.
func updateConfigMapWithRetry(ctx context.Context, c client.Client, namespace string, updateFn func(*OTLPGatewayConfigMap) error) error {
	log := logf.FromContext(ctx)

	for attempt := range maxRetries {
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
			if err := createConfigMap(ctx, c, namespace, string(yamlData)); err != nil {
				if apierrors.IsAlreadyExists(err) {
					log.V(1).Info("configmap created by another controller, retrying", "attempt", attempt+1)
					continue
				}

				return err
			}

			return nil
		}

		if err := updateConfigMap(ctx, c, cm, string(yamlData)); err != nil {
			if apierrors.IsConflict(err) {
				log.V(1).Info("configmap update conflict, retrying", "attempt", attempt+1)
				continue
			}

			return err
		}

		return nil
	}

	return fmt.Errorf("failed to update configmap after %d attempts", maxRetries)
}

// getConfigMap fetches the ConfigMap and returns whether it exists
func getConfigMap(ctx context.Context, c client.Client, namespace string) (*corev1.ConfigMap, bool, error) {
	var cm corev1.ConfigMap

	err := c.Get(ctx, types.NamespacedName{
		Name:      OTLPGatewayConfigMapName,
		Namespace: namespace,
	}, &cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get configmap: %w", err)
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
		return config, fmt.Errorf("failed to unmarshal configmap: %w", err)
	}

	return config, nil
}

// createConfigMap creates a new ConfigMap with the given YAML data
func createConfigMap(ctx context.Context, c client.Client, namespace, yamlData string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OTLPGatewayConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			ConfigMapDataKey: yamlData,
		},
	}

	if err := c.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
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
		return fmt.Errorf("failed to update configmap: %w", err)
	}

	return nil
}
