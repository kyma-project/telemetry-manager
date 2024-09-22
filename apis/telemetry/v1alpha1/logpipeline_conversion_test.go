package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// TestConvertTo tests the ConvertTo method of the LogPipeline
func TestConvertTo(t *testing.T) {
	src := &LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "log-pipeline-test",
		},
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						Include: []string{"default", "kube-system"},
						Exclude: []string{"kube-public"},
						System:  true,
					},
					Containers: InputContainers{
						Include: []string{"nginx", "app"},
						Exclude: []string{"sidecar"},
					},
					KeepAnnotations:  true,
					DropLabels:       true,
					KeepOriginalBody: ptr.To(true),
				},
			},
			Output: Output{
				Custom: "custom-output",
			},
		},
		Status: LogPipelineStatus{
			Conditions: []metav1.Condition{
				{
					Type:    "LogAgentHealthy",
					Status:  "True",
					Reason:  "FluentBitReady",
					Message: "FluentBit is and collecting logs",
				},
			},
			UnsupportedMode: ptr.To(true),
		},
	}

	dst := &v1beta1.LogPipeline{}

	err := src.ConvertTo(dst)
	require.NoError(t, err, "expected no error during ConvertTo")

	require.Equal(t, src.ObjectMeta, dst.ObjectMeta)

	srcAppInput := src.Spec.Input.Application
	dstRuntimeInput := dst.Spec.Input.Runtime

	require.Equal(t, srcAppInput.Namespaces.Include, dstRuntimeInput.Namespaces.Include, "included namespaces mismatch")
	require.Equal(t, srcAppInput.Namespaces.Exclude, dstRuntimeInput.Namespaces.Exclude, "excluded namespaces mismatch")
	require.Equal(t, srcAppInput.Namespaces.System, dstRuntimeInput.Namespaces.System, "system namespaces mismatch")
	require.Equal(t, srcAppInput.Containers.Include, dstRuntimeInput.Containers.Include, "included containers mismatch")
	require.Equal(t, srcAppInput.Containers.Exclude, dstRuntimeInput.Containers.Exclude, "excluded containers mismatch")
	require.Equal(t, srcAppInput.KeepAnnotations, dstRuntimeInput.KeepAnnotations, "keep annotations mismatch")
	require.Equal(t, srcAppInput.DropLabels, dstRuntimeInput.DropLabels, "drop labels mismatch")
	require.Equal(t, srcAppInput.KeepOriginalBody, dstRuntimeInput.KeepOriginalBody, "keep original body mismatch")

	require.Equal(t, src.Spec.Output.Custom, dst.Spec.Output.Custom, "custom output mismatch")
	require.Equal(t, src.Status.UnsupportedMode, dst.Status.UnsupportedMode, "status unsupported mode mismatch")
}

// // TestConvertFrom tests the ConvertFrom method of the LogPipeline
// func TestConvertFrom(t *testing.T) {
// 	src := &v1beta1.LogPipeline{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: "log-pipeline-test",
// 		},
// 		Spec: v1beta1.LogPipelineSpec{
// 			Input: v1beta1.LogPipelineInput{
// 				Runtime: &v1beta1.LogPipelineRuntimeInput{
// 					Namespaces: []string{"default", "kube-system"},
// 					Containers: []string{"nginx", "app"},
// 				},
// 			},
// 			Output: v1beta1.LogPipelineOutput{
// 				Custom: "custom-output",
// 			},
// 		},
// 		Status: v1beta1.LogPipelineStatus("Running"),
// 	}
//
// 	dst := &LogPipeline{}
//
// 	err := dst.ConvertFrom(src)
// 	require.NoError(t, err, "expected no error during ConvertFrom")
//
// 	require.Equal(t, src.ObjectMeta, dst.ObjectMeta, "metadata mismatch")
// 	require.Equal(t, src.Spec.Input.Runtime.Namespaces, dst.Spec.Input.Application.Namespaces, "input namespaces mismatch")
// 	require.Equal(t, src.Spec.Output.Custom, dst.Spec.Output.Custom, "custom output mismatch")
// 	require.Equal(t, string(src.Status), string(dst.Status), "status mismatch")
// }
//
// // TestRoundTripConversion tests round-trip conversion between and v1beta1
// func TestRoundTripConversion(t *testing.T) {
// 	original := &LogPipeline{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: "log-pipeline-test",
// 		},
// 		Spec: LogPipelineSpec{
// 			Input: LogPipelineInputSpec{
// 				Application: ApplicationInput{
// 					Namespaces:       []string{"default", "kube-system"},
// 					Containers:       []string{"nginx", "app"},
// 					KeepAnnotations:  true,
// 					DropLabels:       true,
// 					KeepOriginalBody: true,
// 				},
// 			},
// 			Files:   []FileMount{"file1", "file2"},
// 			Filters: []Filter{"filter1", "filter2"},
// 			Output: LogPipelineOutputSpec{
// 				Custom: "custom-output",
// 				HTTP: &HTTPOutput{
// 					Host:     ValueType{Value: "http://localhost"},
// 					Port:     8080,
// 					Compress: true,
// 				},
// 			},
// 		},
// 		Status: LogPipelineStatus("Running"),
// 	}
//
// 	v1beta := &v1beta1.LogPipeline{}
// 	v1alpha := &LogPipeline{}
//
// 	// Convert original -> v1beta1
// 	err := original.ConvertTo(v1beta)
// 	require.NoError(t, err, "expected no error during ConvertTo")
//
// 	// Convert back v1beta1 -> v1alpha1
// 	err = v1alpha.ConvertFrom(v1beta)
// 	require.NoError(t, err, "expected no error during ConvertFrom")
//
// 	// Assert that the original and the round-tripped version are the same
// 	require.Equal(t, original.ObjectMeta, v1alpha.ObjectMeta, "metadata mismatch after round-trip conversion")
// 	require.Equal(t, original.Spec.Input.Application.Namespaces, v1alpha.Spec.Input.Application.Namespaces, "input namespaces mismatch after round-trip conversion")
// 	require.Equal(t, original.Spec.Output.Custom, v1alpha.Spec.Output.Custom, "custom output mismatch after round-trip conversion")
// 	require.Equal(t, original.Spec.Output.HTTP.Host.Value, v1alpha.Spec.Output.HTTP.Host.Value, "HTTP host mismatch after round-trip conversion")
// 	require.Equal(t, string(original.Status), string(v1alpha.Status), "status mismatch after round-trip conversion")
// }
