package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

func TestDefaultTelemetryInstanceFound(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	_ = operatorv1beta1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&operatorv1beta1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}).Build()

	telemetry, err := GetDefaultTelemetryInstance(ctx, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "default", telemetry.Name)
}

func TestDefaultTelemetryInstanceNotFound(t *testing.T) {
	ctx := t.Context()
	client := fake.NewClientBuilder().Build()

	_, err := GetDefaultTelemetryInstance(ctx, client, "default")
	assert.Error(t, err)
}

func TestGetReplicaCountFromTelemetry(t *testing.T) {
	const (
		testNamespace   = "kyma-system"
		defaultReplicas = int32(2)
	)

	scheme := runtime.NewScheme()
	_ = operatorv1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		telemetry      *operatorv1beta1.Telemetry
		signalType     common.SignalType
		expectedResult int32
	}{
		{
			name:           "telemetry not found returns default",
			telemetry:      nil,
			signalType:     common.SignalTypeTrace,
			expectedResult: defaultReplicas,
		},
		{
			name: "trace gateway with static scaling returns configured replicas",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Trace: &operatorv1beta1.TraceSpec{
						Gateway: operatorv1beta1.GatewaySpec{
							Scaling: operatorv1beta1.Scaling{
								Type: operatorv1beta1.StaticScalingStrategyType,
								Static: &operatorv1beta1.StaticScaling{
									Replicas: 5,
								},
							},
						},
					},
				},
			},
			signalType:     common.SignalTypeTrace,
			expectedResult: 5,
		},
		{
			name: "log gateway with static scaling returns configured replicas",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Log: &operatorv1beta1.LogSpec{
						Gateway: operatorv1beta1.GatewaySpec{
							Scaling: operatorv1beta1.Scaling{
								Type: operatorv1beta1.StaticScalingStrategyType,
								Static: &operatorv1beta1.StaticScaling{
									Replicas: 3,
								},
							},
						},
					},
				},
			},
			signalType:     common.SignalTypeLog,
			expectedResult: 3,
		},
		{
			name: "metric gateway with static scaling returns configured replicas",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Metric: &operatorv1beta1.MetricSpec{
						Gateway: operatorv1beta1.GatewaySpec{
							Scaling: operatorv1beta1.Scaling{
								Type: operatorv1beta1.StaticScalingStrategyType,
								Static: &operatorv1beta1.StaticScaling{
									Replicas: 4,
								},
							},
						},
					},
				},
			},
			signalType:     common.SignalTypeMetric,
			expectedResult: 4,
		},
		{
			name: "gateway spec nil returns default",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{},
			},
			signalType:     common.SignalTypeTrace,
			expectedResult: defaultReplicas,
		},
		{
			name: "static scaling nil returns default",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Trace: &operatorv1beta1.TraceSpec{
						Gateway: operatorv1beta1.GatewaySpec{
							Scaling: operatorv1beta1.Scaling{
								Type: operatorv1beta1.StaticScalingStrategyType,
							},
						},
					},
				},
			},
			signalType:     common.SignalTypeTrace,
			expectedResult: defaultReplicas,
		},
		{
			name: "replicas zero returns default",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Trace: &operatorv1beta1.TraceSpec{
						Gateway: operatorv1beta1.GatewaySpec{
							Scaling: operatorv1beta1.Scaling{
								Type: operatorv1beta1.StaticScalingStrategyType,
								Static: &operatorv1beta1.StaticScaling{
									Replicas: 0,
								},
							},
						},
					},
				},
			},
			signalType:     common.SignalTypeTrace,
			expectedResult: defaultReplicas,
		},
		{
			name: "non-static scaling type returns default",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Trace: &operatorv1beta1.TraceSpec{
						Gateway: operatorv1beta1.GatewaySpec{
							Scaling: operatorv1beta1.Scaling{
								Type: "",
							},
						},
					},
				},
			},
			signalType:     common.SignalTypeTrace,
			expectedResult: defaultReplicas,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.telemetry != nil {
				clientBuilder = clientBuilder.WithObjects(tt.telemetry)
			}

			fakeClient := clientBuilder.Build()

			opts := Options{
				SignalType:                tt.signalType,
				Client:                    fakeClient,
				DefaultTelemetryNamespace: testNamespace,
				DefaultReplicas:           defaultReplicas,
			}

			result := GetReplicaCountFromTelemetry(ctx, opts)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetClusterNameFromTelemetry(t *testing.T) {
	const (
		testNamespace      = "kyma-system"
		defaultClusterName = "${KUBERNETES_SERVICE_HOST}" // fallback from k8sutils.GetGardenerShootInfo
	)

	scheme := runtime.NewScheme()
	_ = operatorv1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		telemetry      *operatorv1beta1.Telemetry
		expectedResult string
	}{
		{
			name:           "telemetry not found returns default cluster name from shoot info",
			telemetry:      nil,
			expectedResult: defaultClusterName,
		},
		{
			name: "enrichments nil returns default cluster name from shoot info",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{},
			},
			expectedResult: defaultClusterName,
		},
		{
			name: "cluster nil returns default cluster name from shoot info",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Enrichments: &operatorv1beta1.EnrichmentSpec{},
				},
			},
			expectedResult: defaultClusterName,
		},
		{
			name: "cluster name empty returns default cluster name from shoot info",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Enrichments: &operatorv1beta1.EnrichmentSpec{
						Cluster: &operatorv1beta1.Cluster{
							Name: "",
						},
					},
				},
			},
			expectedResult: defaultClusterName,
		},
		{
			name: "cluster name configured returns configured value",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
				Spec: operatorv1beta1.TelemetrySpec{
					Enrichments: &operatorv1beta1.EnrichmentSpec{
						Cluster: &operatorv1beta1.Cluster{
							Name: "my-custom-cluster",
						},
					},
				},
			},
			expectedResult: "my-custom-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.telemetry != nil {
				clientBuilder = clientBuilder.WithObjects(tt.telemetry)
			}

			fakeClient := clientBuilder.Build()

			opts := Options{
				Client:                    fakeClient,
				DefaultTelemetryNamespace: testNamespace,
			}

			result := GetClusterNameFromTelemetry(ctx, opts)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetServiceEnrichmentFromTelemetryOrDefault(t *testing.T) {
	const testNamespace = "kyma-system"

	scheme := runtime.NewScheme()
	_ = operatorv1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		telemetry      *operatorv1beta1.Telemetry
		expectedResult string
	}{
		{
			name:           "telemetry not found returns default",
			telemetry:      nil,
			expectedResult: commonresources.AnnotationValueTelemetryServiceEnrichmentDefault,
		},
		{
			name: "annotations nil returns default",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
			},
			expectedResult: commonresources.AnnotationValueTelemetryServiceEnrichmentDefault,
		},
		{
			name: "annotation key not present returns default",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
					Annotations: map[string]string{
						"other-annotation": "some-value",
					},
				},
			},
			expectedResult: commonresources.AnnotationValueTelemetryServiceEnrichmentDefault,
		},
		{
			name: "annotation value otel returns otel",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
					Annotations: map[string]string{
						commonresources.AnnotationKeyTelemetryServiceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
					},
				},
			},
			expectedResult: commonresources.AnnotationValueTelemetryServiceEnrichmentOtel,
		},
		{
			name: "annotation value kyma-legacy returns kyma-legacy",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
					Annotations: map[string]string{
						commonresources.AnnotationKeyTelemetryServiceEnrichment: commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy,
					},
				},
			},
			expectedResult: commonresources.AnnotationValueTelemetryServiceEnrichmentKymaLegacy,
		},
		{
			name: "annotation value invalid returns default",
			telemetry: &operatorv1beta1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
					Annotations: map[string]string{
						commonresources.AnnotationKeyTelemetryServiceEnrichment: "invalid-value",
					},
				},
			},
			expectedResult: commonresources.AnnotationValueTelemetryServiceEnrichmentDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.telemetry != nil {
				clientBuilder = clientBuilder.WithObjects(tt.telemetry)
			}

			fakeClient := clientBuilder.Build()

			opts := Options{
				Client:                    fakeClient,
				DefaultTelemetryNamespace: testNamespace,
			}

			result := GetServiceEnrichmentFromTelemetryOrDefault(ctx, opts)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
