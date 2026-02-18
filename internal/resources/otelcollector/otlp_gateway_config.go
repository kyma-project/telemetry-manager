package otelcollector

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// ReadOTLPGatewayConfig reads and parses the OTLP Gateway ConfigMap from the cluster.
// If the ConfigMap doesn't exist, it returns an empty configuration.
func ReadOTLPGatewayConfig(ctx context.Context, c client.Client, namespace string) (*OTLPGatewayConfigMap, error) {
	var cm corev1.ConfigMap
	err := c.Get(ctx, types.NamespacedName{
		Name:      OTLPGatewayConfigMapName,
		Namespace: namespace,
	}, &cm)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap doesn't exist yet, return empty config
			return &OTLPGatewayConfigMap{}, nil
		}
		return nil, fmt.Errorf("failed to get OTLP Gateway ConfigMap: %w", err)
	}

	// Parse the YAML data
	yamlData, ok := cm.Data[ConfigMapDataKey]
	if !ok || yamlData == "" {
		// No data in ConfigMap, return empty config
		return &OTLPGatewayConfigMap{}, nil
	}

	var config OTLPGatewayConfigMap
	if err := yaml.Unmarshal([]byte(yamlData), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OTLP Gateway ConfigMap data: %w", err)
	}

	return &config, nil
}

// WriteTracePipelineReference adds or updates a TracePipeline reference in the OTLP Gateway ConfigMap.
// It uses optimistic locking with retry to handle concurrent updates safely.
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

// RemoveTracePipelineReference removes a TracePipeline reference from the OTLP Gateway ConfigMap.
// It uses optimistic locking with retry to handle concurrent updates safely.
// This operation is idempotent - if the reference doesn't exist, no error is returned.
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

// updateConfigMapWithRetry implements optimistic locking with retry for ConfigMap updates.
// It reads the current ConfigMap, applies the update function, and writes back.
// If a conflict occurs (409), it retries with fresh data.
func updateConfigMapWithRetry(ctx context.Context, c client.Client, namespace string, updateFn func(*OTLPGatewayConfigMap) error) error {
	log := logf.FromContext(ctx)

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Read current ConfigMap (or create if doesn't exist)
		var cm corev1.ConfigMap
		err := c.Get(ctx, types.NamespacedName{
			Name:      OTLPGatewayConfigMapName,
			Namespace: namespace,
		}, &cm)

		configMapExists := true
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get OTLP Gateway ConfigMap: %w", err)
			}
			configMapExists = false
		}

		// Parse current config
		var config OTLPGatewayConfigMap
		if configMapExists {
			yamlData, ok := cm.Data[ConfigMapDataKey]
			if ok && yamlData != "" {
				if err := yaml.Unmarshal([]byte(yamlData), &config); err != nil {
					return fmt.Errorf("failed to unmarshal ConfigMap data: %w", err)
				}
			}
		}

		// Apply update function
		if err := updateFn(&config); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		// Marshal back to YAML
		yamlData, err := yaml.Marshal(&config)
		if err != nil {
			return fmt.Errorf("failed to marshal ConfigMap data: %w", err)
		}

		// Create or update ConfigMap
		if !configMapExists {
			// Create new ConfigMap
			cm = corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      OTLPGatewayConfigMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					ConfigMapDataKey: string(yamlData),
				},
			}

			if err := c.Create(ctx, &cm); err != nil {
				if apierrors.IsAlreadyExists(err) {
					// Race condition: another controller created it, retry
					log.V(1).Info("ConfigMap was created by another controller, retrying", "attempt", attempt+1)
					continue
				}
				return fmt.Errorf("failed to create ConfigMap: %w", err)
			}

			return nil
		}

		// Update existing ConfigMap
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[ConfigMapDataKey] = string(yamlData)

		if err := c.Update(ctx, &cm); err != nil {
			if apierrors.IsConflict(err) {
				// Conflict: another controller updated it, retry with fresh data
				log.V(1).Info("ConfigMap update conflict, retrying", "attempt", attempt+1)
				continue
			}
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}

		return nil
	}

	return fmt.Errorf("failed to update OTLP Gateway ConfigMap after %d attempts", maxRetries)
}
