package overrides

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadOverrides(t *testing.T) {
	tests := []struct {
		name              string
		configMapData     map[string]string
		expectedOverrides *Config
		expectError       bool
		expectedLogLevel  string
	}{
		{
			name:              "empty configmap",
			configMapData:     map[string]string{},
			expectedOverrides: &Config{},
			expectError:       false,
			expectedLogLevel:  "info",
		},
		{
			name:              "no configmap",
			configMapData:     nil,
			expectedOverrides: &Config{},
			expectError:       false,
			expectedLogLevel:  "info",
		},
		{
			name:              "invalid configmap",
			configMapData:     map[string]string{"test-key": "invalid yaml"},
			expectedOverrides: nil,
			expectError:       true,
			expectedLogLevel:  "info",
		},
		{
			name: "valid configmap",
			configMapData: map[string]string{
				"test-key": `global:
  logLevel: debug
tracing:
  paused: true`,
			},
			expectedOverrides: &Config{
				Global: GlobalConfig{
					LogLevel: "debug",
				},
				Tracing: TracingConfig{
					Paused: true,
				},
			},
			expectError:      false,
			expectedLogLevel: "debug",
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

			atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)
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

			require.Equal(t, atomicLevel.String(), tt.expectedLogLevel)
		})
	}
}
