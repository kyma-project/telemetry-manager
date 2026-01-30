package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// Mock types for testing
type MockPipeline struct {
	Name   string
	Output string
}

type MockReceiver struct {
	Endpoint string `json:"endpoint"`
}

type MockProcessor struct {
	BatchSize int `json:"batch_size"`
}

type MockExporter struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key,omitempty"`
}

func TestComponentBuilder_AddReceiver(t *testing.T) {
	tests := []struct {
		name           string
		componentID    string
		config         any
		expectedConfig any
		expectSkip     bool
	}{
		{
			name:        "adds regular receiver",
			componentID: "otlp",
			config: &MockReceiver{
				Endpoint: "localhost:4317",
			},
			expectedConfig: &MockReceiver{
				Endpoint: "localhost:4317",
			},
		},
		{
			name:        "adds routing connector as receiver",
			componentID: "routing/test",
			config: &MockReceiver{
				Endpoint: "localhost:8080",
			},
			expectedConfig: &MockReceiver{
				Endpoint: "localhost:8080",
			},
		},
		{
			name:        "skips when config is nil",
			componentID: "otlp",
			config:      nil,
			expectSkip:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Receivers:  make(map[string]any),
				Connectors: make(map[string]any),
				Service: Service{
					Pipelines: make(map[string]Pipeline),
				},
			}

			cb := &ComponentBuilder[*MockPipeline]{
				Config:  config,
				EnvVars: make(EnvVars),
			}

			pipeline := &MockPipeline{Name: "test"}
			pipelineID := "logs/test"

			// Create the receiver builder function
			addReceiver := cb.AddReceiver(
				func(p *MockPipeline) string { return tt.componentID },
				func(p *MockPipeline) any { return tt.config },
			)

			// Execute the builder function
			err := addReceiver(context.Background(), pipeline, pipelineID)
			require.NoError(t, err)

			if tt.expectSkip {
				// Should not add anything to receivers or connectors
				require.Empty(t, config.Receivers)
				require.Empty(t, config.Connectors)
				require.Empty(t, config.Service.Pipelines)

				return
			}

			// Verify the component was added to the correct collection
			if isConnector(tt.componentID) {
				require.Contains(t, config.Connectors, tt.componentID)
				require.Equal(t, tt.expectedConfig, config.Connectors[tt.componentID])
				require.Empty(t, config.Receivers)
			} else {
				require.Contains(t, config.Receivers, tt.componentID)
				require.Equal(t, tt.expectedConfig, config.Receivers[tt.componentID])
				require.Empty(t, config.Connectors)
			}

			// Verify pipeline configuration
			require.Contains(t, config.Service.Pipelines, pipelineID)
			pipelineConfig := config.Service.Pipelines[pipelineID]
			require.Contains(t, pipelineConfig.Receivers, tt.componentID)
		})
	}
}

func TestComponentBuilder_AddProcessor(t *testing.T) {
	tests := []struct {
		name           string
		componentID    string
		config         any
		expectedConfig any
		expectSkip     bool
	}{
		{
			name:        "adds processor",
			componentID: "batch",
			config: &MockProcessor{
				BatchSize: 1024,
			},
			expectedConfig: &MockProcessor{
				BatchSize: 1024,
			},
		},
		{
			name:        "skips when config is nil",
			componentID: "memory_limiter",
			config:      nil,
			expectSkip:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Processors: make(map[string]any),
				Service: Service{
					Pipelines: make(map[string]Pipeline),
				},
			}

			cb := &ComponentBuilder[*MockPipeline]{
				Config:  config,
				EnvVars: make(EnvVars),
			}

			pipeline := &MockPipeline{Name: "test"}
			pipelineID := "logs/test"

			// Create the processor builder function
			addProcessor := cb.AddProcessor(
				func(p *MockPipeline) string { return tt.componentID },
				func(p *MockPipeline) any { return tt.config },
			)

			// Execute the builder function
			err := addProcessor(context.Background(), pipeline, pipelineID)
			require.NoError(t, err)

			if tt.expectSkip {
				// Should not add anything to processors
				require.Empty(t, config.Processors)
				require.Empty(t, config.Service.Pipelines)

				return
			}

			// Verify the processor was added
			require.Contains(t, config.Processors, tt.componentID)
			require.Equal(t, tt.expectedConfig, config.Processors[tt.componentID])

			// Verify pipeline configuration
			require.Contains(t, config.Service.Pipelines, pipelineID)
			pipelineConfig := config.Service.Pipelines[pipelineID]
			require.Contains(t, pipelineConfig.Processors, tt.componentID)
		})
	}
}

func TestComponentBuilder_AddExporter(t *testing.T) {
	tests := []struct {
		name           string
		componentID    string
		config         any
		envVars        EnvVars
		expectedConfig any
		expectedEnvs   EnvVars
		expectSkip     bool
		expectError    bool
	}{
		{
			name:        "adds regular exporter with env vars",
			componentID: "otlp_grpc/test",
			config: &MockExporter{
				URL:    "https://api.example.com",
				APIKey: "${API_KEY}",
			},
			envVars: EnvVars{
				"API_KEY": []byte("secret-key-123"),
			},
			expectedConfig: &MockExporter{
				URL:    "https://api.example.com",
				APIKey: "${API_KEY}",
			},
			expectedEnvs: EnvVars{
				"API_KEY": []byte("secret-key-123"),
			},
		},
		{
			name:        "adds forward connector as exporter",
			componentID: "forward/metrics",
			config: &MockExporter{
				URL: "http://internal-service:8080",
			},
			envVars: make(EnvVars),
			expectedConfig: &MockExporter{
				URL: "http://internal-service:8080",
			},
			expectedEnvs: make(EnvVars),
		},
		{
			name:        "skips when config is nil",
			componentID: "otlp_grpc/test",
			config:      nil,
			envVars:     make(EnvVars),
			expectSkip:  true,
		},
		{
			name:        "handles config function error",
			componentID: "otlp_grpc/test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Exporters:  make(map[string]any),
				Connectors: make(map[string]any),
				Service: Service{
					Pipelines: make(map[string]Pipeline),
				},
			}

			cb := &ComponentBuilder[*MockPipeline]{
				Config:  config,
				EnvVars: make(EnvVars),
			}

			pipeline := &MockPipeline{Name: "test"}
			pipelineID := "logs/test"

			var configFunc ExporterComponentConfigFunc[*MockPipeline]
			if tt.expectError {
				configFunc = func(ctx context.Context, p *MockPipeline) (any, EnvVars, error) {
					return nil, nil, fmt.Errorf("config error")
				}
			} else {
				configFunc = func(ctx context.Context, p *MockPipeline) (any, EnvVars, error) {
					return tt.config, tt.envVars, nil
				}
			}

			// Create the exporter builder function
			addExporter := cb.AddExporter(
				func(p *MockPipeline) string { return tt.componentID },
				configFunc,
			)

			// Execute the builder function
			err := addExporter(context.Background(), pipeline, pipelineID)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failed to create exporter config")

				return
			}

			require.NoError(t, err)

			if tt.expectSkip {
				// Should not add anything to exporters or connectors
				require.Empty(t, config.Exporters)
				require.Empty(t, config.Connectors)
				require.Empty(t, config.Service.Pipelines)
				require.Empty(t, cb.EnvVars)

				return
			}

			// Verify the component was added to the correct collection
			if isConnector(tt.componentID) {
				require.Contains(t, config.Connectors, tt.componentID)
				require.Equal(t, tt.expectedConfig, config.Connectors[tt.componentID])
				require.Empty(t, config.Exporters)
			} else {
				require.Contains(t, config.Exporters, tt.componentID)
				require.Equal(t, tt.expectedConfig, config.Exporters[tt.componentID])
				require.Empty(t, config.Connectors)
			}

			// Verify environment variables were merged
			require.Equal(t, tt.expectedEnvs, cb.EnvVars)

			// Verify pipeline configuration
			require.Contains(t, config.Service.Pipelines, pipelineID)
			pipelineConfig := config.Service.Pipelines[pipelineID]
			require.Contains(t, pipelineConfig.Exporters, tt.componentID)
		})
	}
}

func TestComponentBuilder_AddServicePipeline(t *testing.T) {
	t.Run("builds complete pipeline", func(t *testing.T) {
		config := &Config{
			Receivers:  make(map[string]any),
			Processors: make(map[string]any),
			Exporters:  make(map[string]any),
			Service: Service{
				Pipelines: make(map[string]Pipeline),
			},
		}

		cb := &ComponentBuilder[*MockPipeline]{
			Config:  config,
			EnvVars: make(EnvVars),
		}

		pipeline := &MockPipeline{Name: "test"}
		pipelineID := "logs/test"

		// Create component builder functions
		addReceiver := cb.AddReceiver(
			cb.StaticComponentID("otlp"),
			func(p *MockPipeline) any {
				return &MockReceiver{Endpoint: "localhost:4317"}
			},
		)

		addProcessor := cb.AddProcessor(
			cb.StaticComponentID("batch"),
			func(p *MockPipeline) any {
				return &MockProcessor{BatchSize: 1024}
			},
		)

		addExporter := cb.AddExporter(
			func(p *MockPipeline) string { return fmt.Sprintf("otlp/%s", p.Name) },
			func(ctx context.Context, p *MockPipeline) (any, EnvVars, error) {
				return &MockExporter{
					URL:    "https://api.example.com",
					APIKey: "${API_KEY}",
				}, EnvVars{"API_KEY": []byte("secret-123")}, nil
			},
		)

		// Build the complete pipeline
		err := cb.AddServicePipeline(context.Background(), pipeline, pipelineID,
			addReceiver,
			addProcessor,
			addExporter,
		)
		require.NoError(t, err)

		// Verify all components were added
		require.Contains(t, config.Receivers, "otlp")
		require.Contains(t, config.Processors, "batch")
		require.Contains(t, config.Exporters, "otlp/test")

		// Verify pipeline configuration
		require.Contains(t, config.Service.Pipelines, pipelineID)
		pipelineConfig := config.Service.Pipelines[pipelineID]
		require.Equal(t, []string{"otlp"}, pipelineConfig.Receivers)
		require.Equal(t, []string{"batch"}, pipelineConfig.Processors)
		require.Equal(t, []string{"otlp/test"}, pipelineConfig.Exporters)

		// Verify environment variables
		require.Equal(t, EnvVars{"API_KEY": []byte("secret-123")}, cb.EnvVars)
	})

	t.Run("handles component builder error", func(t *testing.T) {
		config := &Config{
			Service: Service{
				Pipelines: make(map[string]Pipeline),
			},
		}

		cb := &ComponentBuilder[*MockPipeline]{
			Config:  config,
			EnvVars: make(EnvVars),
		}

		pipeline := &MockPipeline{Name: "test"}
		pipelineID := "logs/test"

		// Create a failing exporter builder
		failingExporter := cb.AddExporter(
			cb.StaticComponentID("failing"),
			func(ctx context.Context, p *MockPipeline) (any, EnvVars, error) {
				return nil, nil, fmt.Errorf("exporter error")
			},
		)

		// Build pipeline should fail
		err := cb.AddServicePipeline(context.Background(), pipeline, pipelineID,
			failingExporter,
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to add component")
	})
}

func TestComponentBuilder_StaticComponentID(t *testing.T) {
	cb := &ComponentBuilder[*MockPipeline]{
		Config:  &Config{},
		EnvVars: make(EnvVars),
	}

	pipeline := &MockPipeline{Name: "test"}

	componentIDFunc := cb.StaticComponentID("otlp")
	componentID := componentIDFunc(pipeline)

	require.Equal(t, "otlp", componentID)
}

func TestComponentBuilder_ComponentDeduplication(t *testing.T) {
	t.Run("does not duplicate receivers", func(t *testing.T) {
		config := &Config{
			Receivers: make(map[string]any),
			Service: Service{
				Pipelines: make(map[string]Pipeline),
			},
		}

		cb := &ComponentBuilder[*MockPipeline]{
			Config:  config,
			EnvVars: make(EnvVars),
		}

		pipeline := &MockPipeline{Name: "test"}

		addReceiver := cb.AddReceiver(
			cb.StaticComponentID("otlp"),
			func(p *MockPipeline) any {
				return &MockReceiver{Endpoint: "localhost:4317"}
			},
		)

		// Add the same receiver to multiple pipelines
		err := addReceiver(context.Background(), pipeline, "logs/test1")
		require.NoError(t, err)

		err = addReceiver(context.Background(), pipeline, "logs/test2")
		require.NoError(t, err)

		// Should only have one receiver config
		require.Len(t, config.Receivers, 1)
		require.Contains(t, config.Receivers, "otlp")

		// But both pipelines should reference it
		require.Contains(t, config.Service.Pipelines["logs/test1"].Receivers, "otlp")
		require.Contains(t, config.Service.Pipelines["logs/test2"].Receivers, "otlp")
	})

	t.Run("does not duplicate processors", func(t *testing.T) {
		config := &Config{
			Processors: make(map[string]any),
			Service: Service{
				Pipelines: make(map[string]Pipeline),
			},
		}

		cb := &ComponentBuilder[*MockPipeline]{
			Config:  config,
			EnvVars: make(EnvVars),
		}

		pipeline := &MockPipeline{Name: "test"}

		addProcessor := cb.AddProcessor(
			cb.StaticComponentID("batch"),
			func(p *MockPipeline) any {
				return &MockProcessor{BatchSize: 1024}
			},
		)

		// Add the same processor to multiple pipelines
		err := addProcessor(context.Background(), pipeline, "logs/test1")
		require.NoError(t, err)

		err = addProcessor(context.Background(), pipeline, "logs/test2")
		require.NoError(t, err)

		// Should only have one processor config
		require.Len(t, config.Processors, 1)
		require.Contains(t, config.Processors, "batch")

		// But both pipelines should reference it
		require.Contains(t, config.Service.Pipelines["logs/test1"].Processors, "batch")
		require.Contains(t, config.Service.Pipelines["logs/test2"].Processors, "batch")
	})
}
