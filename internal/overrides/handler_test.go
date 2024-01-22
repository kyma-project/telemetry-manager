package overrides

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadOverrides(t *testing.T) {
	tests := []struct {
		name              string
		configMapData     map[string]string
		defaultLevel      zapcore.Level
		expectedOverrides *Config
		expectError       bool
		expectedLogLevel  zapcore.Level
	}{
		{
			name:              "empty configmap",
			configMapData:     map[string]string{},
			defaultLevel:      zapcore.InfoLevel,
			expectedOverrides: &Config{},
			expectError:       false,
			expectedLogLevel:  zapcore.InfoLevel,
		},
		{
			name:              "no configmap",
			configMapData:     nil,
			defaultLevel:      zapcore.InfoLevel,
			expectedOverrides: &Config{},
			expectError:       false,
			expectedLogLevel:  zapcore.InfoLevel,
		},
		{
			name:              "invalid configmap",
			configMapData:     map[string]string{"test-key": "invalid yaml"},
			defaultLevel:      zapcore.InfoLevel,
			expectedOverrides: nil,
			expectError:       true,
			expectedLogLevel:  zapcore.InfoLevel,
		},
		{
			name: "unknown log level",
			configMapData: map[string]string{
				"test-key": `global:
  logLevel: ultradebug`,
			},
			defaultLevel:      zapcore.InfoLevel,
			expectedOverrides: nil,
			expectError:       true,
			expectedLogLevel:  zapcore.InfoLevel,
		},
		{
			name: "valid configmap",
			configMapData: map[string]string{
				"test-key": `global:
  logLevel: debug
tracing:
  paused: true`,
			},
			defaultLevel: zapcore.InfoLevel,
			expectedOverrides: &Config{
				Global: GlobalConfig{
					LogLevel: "debug",
				},
				Tracing: TracingConfig{
					Paused: true,
				},
			},
			expectError:      false,
			expectedLogLevel: zapcore.DebugLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			if tt.configMapData != nil {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
					Data: tt.configMapData,
				}

				err := fakeClient.Create(context.Background(), configMap)
				require.NoError(t, err)
			}

			atomicLevel := zap.NewAtomicLevelAt(tt.defaultLevel)
			handler := New(fakeClient, atomicLevel, HandlerConfig{
				ConfigMapName: types.NamespacedName{
					Name:      "test-configmap",
					Namespace: "test-namespace",
				},
				ConfigMapKey: "test-key",
			})

			overrides, err := handler.LoadOverrides(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOverrides, overrides)
			}

			require.Equal(t, atomicLevel.Level(), tt.expectedLogLevel)
		})
	}
}

func TestLoadOverridesResetsLogLevelIfNoConfigMapFound(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"test-key": `global:
  logLevel: debug
tracing:
  paused: true`,
		},
	}
	err := fakeClient.Create(context.Background(), configMap)
	require.NoError(t, err)

	atomicLevel := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	handler := New(fakeClient, atomicLevel, HandlerConfig{
		ConfigMapName: types.NamespacedName{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		ConfigMapKey: "test-key",
	})

	require.Equal(t, atomicLevel.Level(), zapcore.InfoLevel)

	_, err = handler.LoadOverrides(context.Background())
	require.NoError(t, err)
	require.Equal(t, atomicLevel.Level(), zapcore.DebugLevel, "Should set log level to debug after loading the overrides")

	fakeClient.Delete(context.Background(), configMap)
	_, err = handler.LoadOverrides(context.Background())
	require.NoError(t, err)
	require.Equal(t, atomicLevel.Level(), zapcore.InfoLevel, "Should reset log level back to info after loading empty overrides")
}
