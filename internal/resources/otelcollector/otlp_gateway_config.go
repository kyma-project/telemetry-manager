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

// updateConfigMapWithRetry implements optimistic locking with retry.
// Retries on 409 Conflict errors with fresh data.
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
				return fmt.Errorf("failed to get configmap: %w", err)
			}
			configMapExists = false
		}

		// Parse current config
		var config OTLPGatewayConfigMap
		if configMapExists {
			yamlData, ok := cm.Data[ConfigMapDataKey]
			if ok && yamlData != "" {
				if err := yaml.Unmarshal([]byte(yamlData), &config); err != nil {
					return fmt.Errorf("failed to unmarshal configmap: %w", err)
				}
			}
		}

		if err := updateFn(&config); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		yamlData, err := yaml.Marshal(&config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if !configMapExists {
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
					log.V(1).Info("configmap created by another controller, retrying", "attempt", attempt+1)
					continue
				}
				return fmt.Errorf("failed to create configmap: %w", err)
			}

			return nil
		}

		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[ConfigMapDataKey] = string(yamlData)

		if err := c.Update(ctx, &cm); err != nil {
			if apierrors.IsConflict(err) {
				log.V(1).Info("configmap update conflict, retrying", "attempt", attempt+1)
				continue
			}
			return fmt.Errorf("failed to update configmap: %w", err)
		}

		return nil
	}

	return fmt.Errorf("failed to update configmap after %d attempts", maxRetries)
}
