package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestCreateOutputSectionWithCustomOutput(t *testing.T) {
	expected := `[OUTPUT]
    name                     null
    match                    foo.*
    alias                    foo-null
    retry_limit              300
    storage.total_limit_size 1G

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				Custom: `
    name null`,
			},
		},
	}
	logPipeline.Name = "foo"
	pipelineConfig := PipelineDefaults{FsBufferLimit: "1G"}

	actual := createOutputSection(logPipeline, pipelineConfig)
	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}

func TestCreateOutputSectionWithHTTPOutput(t *testing.T) {
	expected := `[OUTPUT]
    name                     http
    match                    foo.*
    alias                    foo-http
    allow_duplicated_headers true
    format                   yaml
    host                     localhost
    http_passwd              password
    http_user                user
    port                     1234
    retry_limit              300
    storage.total_limit_size 1G
    tls                      on
    tls.verify               on
    uri                      /customindex/kyma

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{
					Dedot:    true,
					Port:     "1234",
					Host:     telemetryv1alpha1.ValueType{Value: "localhost"},
					User:     telemetryv1alpha1.ValueType{Value: "user"},
					Password: telemetryv1alpha1.ValueType{Value: "password"},
					URI:      "/customindex/kyma",
					Format:   "yaml",
				},
			},
		},
	}
	logPipeline.Name = "foo"
	pipelineConfig := PipelineDefaults{FsBufferLimit: "1G"}

	actual := createOutputSection(logPipeline, pipelineConfig)
	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}

func TestCreateOutputSectionWithHTTPOutputWithSecretReference(t *testing.T) {
	expected := `[OUTPUT]
    name                     http
    match                    foo.*
    alias                    foo-http
    allow_duplicated_headers true
    format                   json
    host                     localhost
    http_passwd              ${FOO_MY_NAMESPACE_SECRET_KEY}
    http_user                user
    port                     443
    retry_limit              300
    storage.total_limit_size 1G
    tls                      on
    tls.verify               on
    uri                      /my-uri

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{
					Dedot: true,
					URI:   "/my-uri",
					Host:  telemetryv1alpha1.ValueType{Value: "localhost"},
					User:  telemetryv1alpha1.ValueType{Value: "user"},
					Password: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "secret",
								Key:       "key",
								Namespace: "my-namespace",
							},
						},
					},
				},
			},
		},
	}
	logPipeline.Name = "foo"
	pipelineConfig := PipelineDefaults{FsBufferLimit: "1G"}

	actual := createOutputSection(logPipeline, pipelineConfig)
	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}

func TestCreateOutputSectionWithHTTPOutputWithTLS(t *testing.T) {
	expected := `[OUTPUT]
    name                     http
    match                    foo.*
    alias                    foo-http
    allow_duplicated_headers true
    format                   json
    host                     localhost
    port                     443
    retry_limit              300
    storage.total_limit_size 1G
    tls                      on
    tls.ca_file              /fluent-bit/etc/output-tls-config/foo-ca.crt
    tls.crt_file             /fluent-bit/etc/output-tls-config/foo-cert.crt
    tls.key_file             /fluent-bit/etc/output-tls-config/foo-key.key
    tls.verify               on
    uri                      /my-uri

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{
					Dedot: true,
					URI:   "/my-uri",
					Host:  telemetryv1alpha1.ValueType{Value: "localhost"},
					TLSConfig: telemetryv1alpha1.TLSConfig{
						Disabled:                  false,
						SkipCertificateValidation: false,
						CA:                        &telemetryv1alpha1.ValueType{Value: "fake-ca-value"},
						Cert:                      &telemetryv1alpha1.ValueType{Value: "fake-cert-value"},
						Key:                       &telemetryv1alpha1.ValueType{Value: "fake-key-value"},
					},
				},
			},
		},
	}
	pipelineConfig := PipelineDefaults{FsBufferLimit: "1G"}

	actual := createOutputSection(logPipeline, pipelineConfig)
	require.NotEmpty(t, actual)
	require.Equal(t, expected, actual)
}

func TestResolveValueWithValue(t *testing.T) {
	value := telemetryv1alpha1.ValueType{
		Value: "test",
	}
	resolved := resolveValue(value, "pipeline")
	require.NotEmpty(t, resolved)
	require.Equal(t, resolved, value.Value)
}

func TestResolveValueWithSecretKeyRef(t *testing.T) {
	value := telemetryv1alpha1.ValueType{
		ValueFrom: &telemetryv1alpha1.ValueFromSource{
			SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
				Name:      "test-name",
				Key:       "test-key",
				Namespace: "test-namespace",
			},
		},
	}
	resolved := resolveValue(value, "pipeline")
	require.NotEmpty(t, resolved)
	require.Equal(t, resolved, "${PIPELINE_TEST_NAMESPACE_TEST_NAME_TEST_KEY}")
}
