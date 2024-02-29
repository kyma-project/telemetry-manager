package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{
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
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{Dedot: true},
			},
		},
	}

	actual := createLuaDedotFilter(logPipeline)
	require.Equal(t, "", actual)
}

func TestCreateLuaDedotFilterWithDedotFalse(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{
					Dedot: false,
					Host:  telemetryv1alpha1.ValueType{Value: "localhost"},
				},
			},
		},
	}

	actual := createLuaDedotFilter(logPipeline)
	require.Equal(t, "", actual)
}

func TestMergeSectionsConfig(t *testing.T) {
	expected := `[INPUT]
    name             tail
    alias            foo
    db               /data/flb_foo.db
    exclude_path     /var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log,/var/log/containers/*_*_container1-*.log,/var/log/containers/*_*_container2-*.log
    mem_buf_limit    5MB
    multiline.parser docker, cri, go, python, java
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
    kube_tag_prefix     foo.var.log.containers.
    labels              on
    merge_log           on

[FILTER]
    name  grep
    match foo.*
    regex log aa

[FILTER]
    name   lua
    match  foo.*
    call   kubernetes_map_keys
    script /fluent-bit/scripts/filter-script.lua

[OUTPUT]
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
    tls.verify               on

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					Containers: telemetryv1alpha1.InputContainers{
						Exclude: []string{"container1", "container2"},
					},
					Namespaces: telemetryv1alpha1.InputNamespaces{
						System: true,
					},
					KeepAnnotations: true,
					DropLabels:      false,
				},
			},
			Filters: []telemetryv1alpha1.Filter{
				{
					Custom: `
						name grep
						regex log aa
					`,
				},
			},
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{
					Dedot: true,
					Host: telemetryv1alpha1.ValueType{
						Value: "localhost",
					},
				},
			},
		},
	}
	logPipeline.Name = "foo"
	defaults := PipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
		FsBufferLimit:     "1G",
	}

	actual, err := BuildFluentBitConfig(logPipeline, defaults)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestMergeSectionsConfigCustomOutput(t *testing.T) {
	expected := `[INPUT]
    name             tail
    alias            foo
    db               /data/flb_foo.db
    exclude_path     /var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log
    mem_buf_limit    5MB
    multiline.parser docker, cri, go, python, java
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
    kube_tag_prefix     foo.var.log.containers.
    labels              on
    merge_log           on

[OUTPUT]
    name                     stdout
    match                    foo.*
    alias                    foo-stdout
    retry_limit              300
    storage.total_limit_size 1G

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					KeepAnnotations: true,
					DropLabels:      false,
					Namespaces: telemetryv1alpha1.InputNamespaces{
						System: true,
					},
				},
			},
			Output: telemetryv1alpha1.Output{
				Custom: `
    name stdout`,
			},
		},
	}
	logPipeline.Name = "foo"
	defaults := PipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
		FsBufferLimit:     "1G",
	}

	actual, err := BuildFluentBitConfig(logPipeline, defaults)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestMergeSectionsConfigWithMissingOutput(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{}
	logPipeline.Name = "foo"
	defaults := PipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
		FsBufferLimit:     "1G",
	}

	actual, err := BuildFluentBitConfig(logPipeline, defaults)
	require.Error(t, err)
	require.Empty(t, actual)
}
