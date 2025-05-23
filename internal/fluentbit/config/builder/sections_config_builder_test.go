package builder

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestCreateRecordModifierFilter(t *testing.T) {
	expected := `[FILTER]
    name   record_modifier
    match  foo.*
    record cluster_identifier ${KUBERNETES_SERVICE_HOST}

`
	logPipeline := &telemetryv1alpha1.LogPipeline{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}

	actual := createRecordModifierFilter(logPipeline)
	require.Equal(t, expected, actual, "Fluent Bit Permanent parser config is invalid")
}

func TestCreateLuaDedotFilterWithDefinedHostAndDedotSet(t *testing.T) {
	expected := `[FILTER]
    name   lua
    match  foo.*
    call   kubernetes_map_keys
    script /fluent-bit/scripts/filter-script.lua

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Dedot: true,
					Host:  telemetryv1alpha1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createLuaDedotFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestCreateLuaDedotFilterWithUndefinedHost(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{Dedot: true},
			},
		},
	}

	actual := createLuaDedotFilter(logPipeline)
	require.Equal(t, "", actual)
}

func TestCreateLuaDedotFilterWithDedotFalse(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Dedot: false,
					Host:  telemetryv1alpha1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createLuaDedotFilter(logPipeline)
	require.Equal(t, "", actual)
}

func TestCreateLuaEnrichAppNameFilter(t *testing.T) {
	expected := `[FILTER]
    name   lua
    match  foo.*
    call   enrich_app_name
    script /fluent-bit/scripts/filter-script.lua

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Host: telemetryv1alpha1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createLuaEnrichAppNameFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestCreateTimestampAndAppNameModifyFilter(t *testing.T) {
	expected := `[FILTER]
    name  modify
    match foo.*
    copy  time @timestamp

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Host: telemetryv1alpha1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createTimestampAndAppNameModifyFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestMergeSectionsConfig(t *testing.T) {
	excludePath := strings.Join([]string{
		"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
		"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
		"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
		"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
		"/var/log/containers/*_*_container1-*.log",
		"/var/log/containers/*_*_container2-*.log",
	}, ",")
	expected := fmt.Sprintf(`[INPUT]
    name             tail
    alias            foo
    db               /data/flb_foo.db
    exclude_path     %s
    mem_buf_limit    5MB
    multiline.parser cri
    path             /var/log/containers/*_*_*-*.log
    read_from_head   true
    skip_long_lines  on
    storage.type     filesystem
    tag              foo.*

[FILTER]
    name             multiline
    match            foo.*
    multiline.parser java

[FILTER]
    name   record_modifier
    match  foo.*
    record cluster_identifier ${KUBERNETES_SERVICE_HOST}

[FILTER]
    name                kubernetes
    match               foo.*
    annotations         on
    buffer_size         1MB
    k8s-logging.exclude off
    k8s-logging.parser  on
    keep_log            on
    kube_tag_prefix     foo.var.log.containers.
    labels              on
    merge_log           on

[FILTER]
    name  modify
    match foo.*
    copy  time @timestamp

[FILTER]
    name  grep
    match foo.*
    regex log aa

[FILTER]
    name   lua
    match  foo.*
    call   enrich_app_name
    script /fluent-bit/scripts/filter-script.lua

[FILTER]
    name   lua
    match  foo.*
    call   kubernetes_map_keys
    script /fluent-bit/scripts/filter-script.lua

[OUTPUT]
    name                     http
    match                    foo.*
    alias                    foo
    allow_duplicated_headers true
    format                   json
    host                     localhost
    json_date_format         iso8601
    port                     443
    retry_limit              300
    storage.total_limit_size 1G
    tls                      on
    tls.verify               on

`, excludePath)
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Containers: telemetryv1alpha1.LogPipelineContainerSelector{
						Exclude: []string{"container1", "container2"},
					},
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						System: true,
					},
					KeepAnnotations:  ptr.To(true),
					DropLabels:       ptr.To(false),
					KeepOriginalBody: ptr.To(true),
				},
			},
			Filters: []telemetryv1alpha1.LogPipelineFilter{
				{
					Custom: `
						name grep
						regex log aa
					`,
				},
				{
					Custom: `
						name multiline
						multiline.parser java
					`,
				},
			},
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Dedot: true,
					Host: telemetryv1alpha1.ValueType{
						Value: "localhost",
					},
				},
			},
		},
	}
	logPipeline.Name = "foo"
	defaults := pipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
		FsBufferLimit:     "1G",
	}

	actual, err := buildFluentBitSectionsConfig(logPipeline, builderConfig{pipelineDefaults: defaults})
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestMergeSectionsConfigCustomOutput(t *testing.T) {
	excludePath := strings.Join([]string{
		"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
		"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
		"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
	}, ",")
	expected := fmt.Sprintf(`[INPUT]
    name             tail
    alias            foo
    db               /data/flb_foo.db
    exclude_path     %s
    mem_buf_limit    5MB
    multiline.parser cri
    path             /var/log/containers/*_*_*-*.log
    read_from_head   true
    skip_long_lines  on
    storage.type     filesystem
    tag              foo.*

[FILTER]
    name   record_modifier
    match  foo.*
    record cluster_identifier ${KUBERNETES_SERVICE_HOST}

[FILTER]
    name                kubernetes
    match               foo.*
    annotations         on
    buffer_size         1MB
    k8s-logging.exclude off
    k8s-logging.parser  on
    keep_log            on
    kube_tag_prefix     foo.var.log.containers.
    labels              on
    merge_log           on

[OUTPUT]
    name                     stdout
    match                    foo.*
    alias                    foo
    retry_limit              300
    storage.total_limit_size 1G

`, excludePath)
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					KeepAnnotations:  ptr.To(true),
					DropLabels:       ptr.To(false),
					KeepOriginalBody: ptr.To(true),
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						System: true,
					},
				},
			},
			Output: telemetryv1alpha1.LogPipelineOutput{
				Custom: `
    name stdout`,
			},
		},
	}
	logPipeline.Name = "foo"
	defaults := pipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
		FsBufferLimit:     "1G",
	}
	config := builderConfig{
		pipelineDefaults: defaults,
		collectAgentLogs: true,
	}

	actual, err := buildFluentBitSectionsConfig(logPipeline, config)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestMergeSectionsConfigWithMissingOutput(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{}
	logPipeline.Name = "foo"
	defaults := pipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
		FsBufferLimit:     "1G",
	}

	actual, err := buildFluentBitSectionsConfig(logPipeline, builderConfig{pipelineDefaults: defaults})
	require.Error(t, err)
	require.Empty(t, actual)
}

func TestBuildFluentBitConfig_Validation(t *testing.T) {
	type args struct {
		pipeline *telemetryv1alpha1.LogPipeline
		config   builderConfig
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr error
	}{
		{
			name: "Should return error when pipeline mode is not FluentBit",
			args: args{
				pipeline: func() *telemetryv1alpha1.LogPipeline {
					lp := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
					return &lp
				}(),
			},
			want:    "",
			wantErr: errInvalidPipelineDefinition,
		},
		{
			name: "Should return error when input OTLP is defined",
			args: args{
				pipeline: func() *telemetryv1alpha1.LogPipeline {
					lp := testutils.NewLogPipelineBuilder().WithHTTPOutput().WithOTLPInput(true).Build()
					return &lp
				}(),
			},
			want:    "",
			wantErr: errInvalidPipelineDefinition,
		},
		{
			name: "Should return error when output plugin is not defined",
			args: args{
				pipeline: func() *telemetryv1alpha1.LogPipeline {
					lp := testutils.NewLogPipelineBuilder().Build()
					lp.Spec.Output = telemetryv1alpha1.LogPipelineOutput{}
					return &lp
				}(),
			},
			want:    "",
			wantErr: errInvalidPipelineDefinition,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildFluentBitSectionsConfig(tt.args.pipeline, tt.args.config)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			}

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("buildFluentBitSectionsConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
