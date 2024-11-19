package metricpipeline

import (
	"context"
	"encoding/json"
	"testing"


	"github.com/stretchr/testify/require"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should add runtime input defaults when runtime input enabled", func(t *testing.T) {
		metricPipeline := testutils.NewMetricPipelineBuilder().WithName("default").WithRuntimeInput(true).Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, metricPipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 3, "should have 3 patches")

		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/output/otlp/protocol",
			Value:     "grpc",
		}, "should have added default OTLP protocol")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/namespaces",
			Value:     map[string]interface{}{"exclude": []interface{}{"kyma-system", "kube-system", "istio-system", "compass-system"}},
		}, "should have added default excluded namespaces")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources",
			Value: map[string]interface{}{
				"container":   map[string]interface{}{"enabled": true},
				"daemonset":   map[string]interface{}{"enabled": true},
				"deployment":  map[string]interface{}{"enabled": true},
				"job":         map[string]interface{}{"enabled": true},
				"node":        map[string]interface{}{"enabled": true},
				"pod":         map[string]interface{}{"enabled": true},
				"statefulset": map[string]interface{}{"enabled": true},
				"volume":      map[string]interface{}{"enabled": true},
			}}, "should have added default runtime input resources")
	})

	t.Run("should add runtime input resources defaults when runtime input enabled, except runtime Job configuration", func(t *testing.T) {
		metricPipeline := testutils.NewMetricPipelineBuilder().WithName("default").WithRuntimeInput(true).WithRuntimeInputJobMetrics(false).Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, metricPipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 9, "should have 9 patches")

		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/output/otlp/protocol",
			Value:     "grpc",
		}, "should have added default OTLP protocol")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/namespaces",
			Value:     map[string]interface{}{"exclude": []interface{}{"kyma-system", "kube-system", "istio-system", "compass-system"}},
		}, "should have added default excluded namespaces")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources/container",
			Value:     map[string]interface{}{"enabled": true}}, "should have added default runtime input resources")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources/daemonset",
			Value:     map[string]interface{}{"enabled": true}}, "should have added default runtime input resources")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources/deployment",
			Value:     map[string]interface{}{"enabled": true}}, "should have added default runtime input resources")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources/volume",
			Value:     map[string]interface{}{"enabled": true}}, "should have added default runtime input resources")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources/node",
			Value:     map[string]interface{}{"enabled": true}}, "should have added default runtime input resources")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources/pod",
			Value:     map[string]interface{}{"enabled": true}}, "should have added default runtime input resources")
		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/runtime/resources/statefulset",
			Value:     map[string]interface{}{"enabled": true}}, "should have added default runtime input resources")
	})

	t.Run("should not have default OTLP output protocol when protocol configured", func(t *testing.T) {
		metricPipeline := testutils.NewMetricPipelineBuilder().WithName("default-grpc").WithOTLPOutput(testutils.OTLPEndpoint("test-endpoint:4817"), testutils.OTLPProtocol("http")).WithRuntimeInput(true).Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, metricPipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 2, "should have 2 patches")

		require.NotContains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/output/otlp/protocol",
		}, "should not have added default OTLP protocol")
	})

	t.Run("should add prometheus defaults when prometheus input enabled", func(t *testing.T) {
		metricPipeline := testutils.NewMetricPipelineBuilder().WithName("default-prometheus").WithOTLPOutput(testutils.OTLPEndpoint("test-endpoint:4817"), testutils.OTLPProtocol("http")).WithPrometheusInput(true).Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, metricPipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 1, "should have 1 patches")

		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/prometheus/namespaces",
			Value:     map[string]interface{}{"exclude": []interface{}{"kyma-system", "kube-system", "istio-system", "compass-system"}},
		}, "should have added default excluded namespaces")
	})

	t.Run("should add istio input defaults when istio input enabled", func(t *testing.T) {
		metricPipeline := testutils.NewMetricPipelineBuilder().WithName("default-istio").WithOTLPOutput(testutils.OTLPEndpoint("test-endpoint:4817"), testutils.OTLPProtocol("http")).WithIstioInput(true).Build()

		sut := NewDefaultingWebhookHandler(scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, metricPipeline))

		require.True(t, response.Allowed)
		require.Len(t, response.Patches, 1, "should have 1 patches")

		require.Contains(t, response.Patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/input/istio/namespaces",
			Value:     map[string]interface{}{"exclude": []interface{}{"kyma-system", "kube-system", "istio-system", "compass-system"}},
		}, "should have added default excluded namespaces")
	})
}

func admissionRequestFrom(t *testing.T, metricPipeline telemetryv1alpha1.MetricPipeline) admission.Request {
	t.Helper()

	pipelineJSON, err := json.Marshal(metricPipeline)
	if err != nil {
		t.Fatalf("failed to marshal metric pipeline: %v", err)
	}

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: pipelineJSON},
		},
	}
}
