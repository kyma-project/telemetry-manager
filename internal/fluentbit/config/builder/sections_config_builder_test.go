package builder

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestCreateRecordModifierFilter(t *testing.T) {
	expected := `[FILTER]
    name   record_modifier
    match  foo.*
    record cluster_identifier test-cluster-name

`
	logPipeline := &telemetryv1beta1.LogPipeline{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}

	actual := createRecordModifierFilter(logPipeline, "test-cluster-name")
	require.Equal(t, expected, actual, "Fluent Bit Permanent parser config is invalid")
}

func TestCreateLuaFilterWithDefinedHostAndDedot(t *testing.T) {
	expected := `[FILTER]
    name   lua
    match  foo.*
    call   dedot_and_enrich_app_name
    script /fluent-bit/scripts/filter-script.lua

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
					Dedot: true,
					Host:  telemetryv1beta1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createLuaFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestCreateLuaFilterWithUndefinedHostAndDedot(t *testing.T) {
	logPipeline := &telemetryv1beta1.LogPipeline{
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{Dedot: true},
			},
		},
	}

	actual := createLuaFilter(logPipeline)
	require.Equal(t, "", actual)
}

func TestCreateLuaFilterWithDedotFalse(t *testing.T) {
	expected := `[FILTER]
    name   lua
    match  foo.*
    call   enrich_app_name
    script /fluent-bit/scripts/filter-script.lua

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
					Dedot: false,
					Host:  telemetryv1beta1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createLuaFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestCreateTimestampModifyFilter(t *testing.T) {
	expected := `[FILTER]
    name  modify
    match foo.*
    copy  time @timestamp

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
					Host: telemetryv1beta1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createTimestampModifyFilter(logPipeline)
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
    record cluster_identifier test-cluster-name

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
    call   dedot_and_enrich_app_name
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
	logPipeline := &telemetryv1beta1.LogPipeline{
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{
				Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
					Containers: &telemetryv1beta1.LogPipelineContainerSelector{
						Exclude: []string{"container1", "container2"},
					},
					Namespaces:               &telemetryv1beta1.NamespaceSelector{},
					FluentBitKeepAnnotations: ptr.To(true),
					FluentBitDropLabels:      ptr.To(false),
					KeepOriginalBody:         ptr.To(true),
				},
			},
			FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
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
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
					Dedot: true,
					Host: telemetryv1beta1.ValueType{
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

	actual, err := buildFluentBitSectionsConfig(logPipeline, builderConfig{pipelineDefaults: defaults}, "test-cluster-name")
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
    record cluster_identifier test-cluster-name

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
	logPipeline := &telemetryv1beta1.LogPipeline{
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{
				Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
					FluentBitKeepAnnotations: ptr.To(true),
					FluentBitDropLabels:      ptr.To(false),
					KeepOriginalBody:         ptr.To(true),
					Namespaces:               &telemetryv1beta1.NamespaceSelector{},
				},
			},
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitCustom: `
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

	actual, err := buildFluentBitSectionsConfig(logPipeline, config, "test-cluster-name")
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
