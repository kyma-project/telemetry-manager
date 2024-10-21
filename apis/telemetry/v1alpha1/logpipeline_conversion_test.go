package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestConvertTo(t *testing.T) {
	src := &LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "log-pipeline-test",
		},
		Spec: LogPipelineSpec{
			Input: Input{
				Application: &ApplicationInput{
					Enabled: ptr.To(true),
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
			Files: []FileMount{
				{Name: "file1", Content: "file1-content"},
			},
			Filters: []Filter{
				{Custom: "name stdout"},
			},
			Output: Output{
				Custom: "custom-output",
				HTTP: &HTTPOutput{
					Host: ValueType{
						Value: "http://localhost",
					},
					User: ValueType{
						Value: "user",
					},
					Password: ValueType{
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name:      "secret-name",
								Namespace: "secret-namespace",
								Key:       "secret-key",
							},
						},
					},
					URI:      "/ingest/v1beta1/logs",
					Port:     "8080",
					Compress: "on",
					Format:   "json",
					TLSConfig: TLSConfig{
						SkipCertificateValidation: true,
						CA: &ValueType{
							Value: "ca",
						},
						Cert: &ValueType{
							Value: "cert",
						},
						Key: &ValueType{
							Value: "key",
						},
					},
					Dedot: true,
				},
				Otlp: &OtlpOutput{
					Protocol: OtlpProtocolGRPC,
					Endpoint: ValueType{
						Value: "localhost:4317",
					},
					Path: "/v1/logs",
					Authentication: &AuthenticationOptions{
						Basic: &BasicAuthOptions{
							User: ValueType{
								Value: "user",
							},
							Password: ValueType{
								Value: "password",
							},
						},
					},
					Headers: []Header{
						{
							Name: "header1",
							ValueType: ValueType{
								Value: "value1",
							},
							Prefix: "prefix1",
						},
						{
							Name: "header2",
							ValueType: ValueType{
								Value: "value2",
							},
							Prefix: "prefix2",
						},
					},
					TLS: &OtlpTLS{
						Insecure:           true,
						InsecureSkipVerify: true,
						CA: &ValueType{
							Value: "ca",
						},
						Cert: &ValueType{
							Value: "cert",
						},
						Key: &ValueType{
							Value: "key",
						},
					},
				},
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

	dst := &telemetryv1beta1.LogPipeline{}

	err := src.ConvertTo(dst)
	require.NoError(t, err)

	requireLogPipelinesEquivalent(t, src, dst)

	srcAfterRoundTrip := &LogPipeline{}
	err = srcAfterRoundTrip.ConvertFrom(dst)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(src, srcAfterRoundTrip), "expected source be equal to itself after round-trip")
}

func TestConvertFrom(t *testing.T) {
	src := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "log-pipeline-test",
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{
				Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
					Enabled: ptr.To(true),
					Namespaces: telemetryv1beta1.LogPipelineInputNamespaces{
						Include: []string{"default", "kube-system"},
						Exclude: []string{"kube-public"},
						System:  true,
					},
					Containers: telemetryv1beta1.LogPipelineInputContainers{
						Include: []string{"nginx", "app"},
						Exclude: []string{"sidecar"},
					},
					KeepAnnotations:  true,
					DropLabels:       true,
					KeepOriginalBody: ptr.To(true),
				},
			},
			Files: []telemetryv1beta1.LogPipelineFileMount{
				{Name: "file1", Content: "file1-content"},
			},
			Filters: []telemetryv1beta1.LogPipelineFilter{
				{Custom: "name stdout"},
			},
			Output: telemetryv1beta1.LogPipelineOutput{
				Custom: "custom-output",
				HTTP: &telemetryv1beta1.LogPipelineHTTPOutput{
					Host: telemetryv1beta1.ValueType{
						Value: "http://localhost",
					},
					User: telemetryv1beta1.ValueType{
						Value: "user",
					},
					Password: telemetryv1beta1.ValueType{
						ValueFrom: &telemetryv1beta1.ValueFromSource{
							SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
								Name:      "secret-name",
								Namespace: "secret-namespace",
								Key:       "secret-key",
							},
						},
					},
					URI:      "/ingest/v1beta1/logs",
					Port:     "8080",
					Compress: "on",
					Format:   "json",
					TLSConfig: telemetryv1beta1.OutputTLS{
						SkipCertificateValidation: true,
						CA: &telemetryv1beta1.ValueType{
							Value: "ca",
						},
						Cert: &telemetryv1beta1.ValueType{
							Value: "cert",
						},
						Key: &telemetryv1beta1.ValueType{
							Value: "key",
						},
					},
					Dedot: true,
				},
				OTLP: &telemetryv1beta1.OTLPOutput{
					Protocol: telemetryv1beta1.OTLPProtocolGRPC,
					Endpoint: telemetryv1beta1.ValueType{Value: "localhost:4317"},
					Path:     "/v1/logs",
					Authentication: &telemetryv1beta1.AuthenticationOptions{Basic: &telemetryv1beta1.BasicAuthOptions{
						User: telemetryv1beta1.ValueType{
							Value: "user",
						},
						Password: telemetryv1beta1.ValueType{
							Value: "password",
						},
					}},
					Headers: []telemetryv1beta1.Header{
						{
							Name: "header1",
							ValueType: telemetryv1beta1.ValueType{
								Value: "value1",
							},
							Prefix: "prefix1",
						},
						{
							Name: "header2",
							ValueType: telemetryv1beta1.ValueType{
								Value: "value2",
							},
							Prefix: "prefix2",
						},
					},
					TLS: &telemetryv1beta1.OutputTLS{
						Disabled:                  true,
						SkipCertificateValidation: true,
						CA:                        &telemetryv1beta1.ValueType{Value: "ca"},
						Cert:                      &telemetryv1beta1.ValueType{Value: "cert"},
						Key:                       &telemetryv1beta1.ValueType{Value: "key"},
					},
				},
			},
		},
		Status: telemetryv1beta1.LogPipelineStatus{
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

	dst := &LogPipeline{}

	err := dst.ConvertFrom(src)
	require.NoError(t, err, "expected no error during ConvertTo")

	requireLogPipelinesEquivalent(t, dst, src)

	srcAfterRoundTrip := &telemetryv1beta1.LogPipeline{}
	err = dst.ConvertTo(srcAfterRoundTrip)
	require.NoError(t, err, "expected no error during ConvertFrom (round-trip)")
	require.Empty(t, cmp.Diff(src, srcAfterRoundTrip), "expected source be equal to itself after round-trip")
}

func requireLogPipelinesEquivalent(t *testing.T, x *LogPipeline, y *telemetryv1beta1.LogPipeline) {
	require.Equal(t, x.ObjectMeta, y.ObjectMeta)

	xAppInput := x.Spec.Input.Application
	yRuntimeInput := y.Spec.Input.Runtime
	require.Equal(t, xAppInput.Namespaces.Include, yRuntimeInput.Namespaces.Include, "included namespaces mismatch")
	require.Equal(t, xAppInput.Namespaces.Exclude, yRuntimeInput.Namespaces.Exclude, "excluded namespaces mismatch")
	require.Equal(t, xAppInput.Namespaces.System, yRuntimeInput.Namespaces.System, "system namespaces mismatch")
	require.Equal(t, xAppInput.Containers.Include, yRuntimeInput.Containers.Include, "included containers mismatch")
	require.Equal(t, xAppInput.Containers.Exclude, yRuntimeInput.Containers.Exclude, "excluded containers mismatch")
	require.Equal(t, xAppInput.KeepAnnotations, yRuntimeInput.KeepAnnotations, "keep annotations mismatch")
	require.Equal(t, xAppInput.DropLabels, yRuntimeInput.DropLabels, "drop labels mismatch")
	require.Equal(t, xAppInput.KeepOriginalBody, yRuntimeInput.KeepOriginalBody, "keep original body mismatch")

	require.Len(t, y.Spec.Files, 1, "expected one file")
	require.Equal(t, x.Spec.Files[0].Name, y.Spec.Files[0].Name, "file name mismatch")

	require.Len(t, y.Spec.Filters, 1, "expected one filter")
	require.Equal(t, x.Spec.Filters[0].Custom, y.Spec.Filters[0].Custom, "custom filter mismatch")

	require.Equal(t, x.Spec.Output.Custom, y.Spec.Output.Custom, "custom output mismatch")

	xHTTP := x.Spec.Output.HTTP
	yHTTP := y.Spec.Output.HTTP
	require.Equal(t, xHTTP.Host.Value, yHTTP.Host.Value, "HTTP host mismatch")
	require.Equal(t, xHTTP.User.Value, yHTTP.User.Value, "HTTP user mismatch")
	require.Equal(t, xHTTP.Password.ValueFrom.SecretKeyRef.Name, yHTTP.Password.ValueFrom.SecretKeyRef.Name, "HTTP password secret name mismatch")
	require.Equal(t, xHTTP.Password.ValueFrom.SecretKeyRef.Namespace, yHTTP.Password.ValueFrom.SecretKeyRef.Namespace, "HTTP password secret namespace mismatch")
	require.Equal(t, xHTTP.Password.ValueFrom.SecretKeyRef.Key, yHTTP.Password.ValueFrom.SecretKeyRef.Key, "HTTP password secret key mismatch")
	require.Equal(t, xHTTP.URI, yHTTP.URI, "HTTP URI mismatch")
	require.Equal(t, xHTTP.Port, yHTTP.Port, "HTTP port mismatch")
	require.Equal(t, xHTTP.Compress, yHTTP.Compress, "HTTP compress mismatch")
	require.Equal(t, xHTTP.Format, yHTTP.Format, "HTTP format mismatch")
	require.Equal(t, xHTTP.TLSConfig.SkipCertificateValidation, yHTTP.TLSConfig.SkipCertificateValidation, "HTTP TLS skip certificate validation mismatch")
	require.Equal(t, xHTTP.TLSConfig.CA.Value, yHTTP.TLSConfig.CA.Value, "HTTP TLS CA mismatch")
	require.Equal(t, xHTTP.TLSConfig.Cert.Value, yHTTP.TLSConfig.Cert.Value, "HTTP TLS cert mismatch")
	require.Equal(t, xHTTP.TLSConfig.Key.Value, yHTTP.TLSConfig.Key.Value, "HTTP TLS key mismatch")

	xOTLP := x.Spec.Output.Otlp
	yOTLP := y.Spec.Output.OTLP

	require.NotNil(t, xOTLP, "expected OTLP output")
	require.NotNil(t, yOTLP, "expected OTLP output")
	require.Equal(t, xOTLP.Protocol, string(yOTLP.Protocol), "OTLP protocol mismatch")
	require.Equal(t, xOTLP.Endpoint.Value, yOTLP.Endpoint.Value, "OTLP endpoint mismatch")
	require.Equal(t, xOTLP.Path, yOTLP.Path, "OTLP path mismatch")
	require.Equal(t, xOTLP.Authentication.Basic.User.Value, yOTLP.Authentication.Basic.User.Value, "OTLP basic auth user mismatch")
	require.Equal(t, xOTLP.Authentication.Basic.Password.Value, yOTLP.Authentication.Basic.Password.Value, "OTLP basic auth password mismatch")
	require.Len(t, yOTLP.Headers, 2, "expected two headers")
	require.Equal(t, xOTLP.Headers[0].Name, yOTLP.Headers[0].Name, "OTLP header name mismatch")
	require.Equal(t, xOTLP.Headers[0].ValueType.Value, yOTLP.Headers[0].ValueType.Value, "OTLP header value mismatch")
	require.Equal(t, xOTLP.Headers[0].Prefix, yOTLP.Headers[0].Prefix, "OTLP header prefix mismatch")
	require.Equal(t, xOTLP.Headers[1].Name, yOTLP.Headers[1].Name, "OTLP header name mismatch")
	require.Equal(t, xOTLP.Headers[1].ValueType.Value, yOTLP.Headers[1].ValueType.Value, "OTLP header value mismatch")
	require.Equal(t, xOTLP.Headers[1].Prefix, yOTLP.Headers[1].Prefix, "OTLP header prefix mismatch")
	require.Equal(t, xOTLP.TLS.Insecure, yOTLP.TLS.Disabled, "OTLP TLS insecure mismatch")
	require.Equal(t, xOTLP.TLS.InsecureSkipVerify, yOTLP.TLS.SkipCertificateValidation, "OTLP TLS insecure skip verify mismatch")
	require.Equal(t, xOTLP.TLS.CA.Value, yOTLP.TLS.CA.Value, "OTLP TLS CA mismatch")
	require.Equal(t, xOTLP.TLS.Cert.Value, yOTLP.TLS.Cert.Value, "OTLP TLS cert mismatch")
	require.Equal(t, xOTLP.TLS.Key.Value, yOTLP.TLS.Key.Value, "OTLP TLS key mismatch")

	require.Equal(t, x.Status.UnsupportedMode, y.Status.UnsupportedMode, "status unsupported mode mismatch")
	require.ElementsMatch(t, x.Status.Conditions, y.Status.Conditions, "status conditions mismatch")
}
