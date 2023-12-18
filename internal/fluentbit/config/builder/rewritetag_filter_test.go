package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestCreateRewriteTagFilterIncludeContainers(t *testing.T) {
	pipelineConfig := PipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
	}

	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "logpipeline1",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{Application: telemetryv1alpha1.ApplicationInput{
				Containers: telemetryv1alpha1.InputContainers{
					Include: []string{"container1", "container2"}}}}}}

	expected := `[FILTER]
    name                  rewrite_tag
    match                 kube.*
    emitter_mem_buf_limit 10M
    emitter_name          logpipeline1
    emitter_storage.type  filesystem
    rule                  $kubernetes['container_name'] "^(container1|container2)$" logpipeline1.$TAG true

`
	actual := createRewriteTagFilter(logPipeline, pipelineConfig)
	require.Equal(t, expected, actual)
}

func TestCreateRewriteTagFilterExcludeContainers(t *testing.T) {
	pipelineConfig := PipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
	}

	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "logpipeline1",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{Application: telemetryv1alpha1.ApplicationInput{
				Containers: telemetryv1alpha1.InputContainers{
					Exclude: []string{"container1", "container2"}}}}}}

	expected := `[FILTER]
    name                  rewrite_tag
    match                 kube.*
    emitter_mem_buf_limit 10M
    emitter_name          logpipeline1
    emitter_storage.type  filesystem
    rule                  $kubernetes['container_name'] "^(?!container1$|container2$).*" logpipeline1.$TAG true

`
	actual := createRewriteTagFilter(logPipeline, pipelineConfig)
	require.Equal(t, expected, actual)
}

func TestCreateRewriteTagFilterWithCustomOutput(t *testing.T) {
	pipelineConfig := PipelineDefaults{
		InputTag:          "kube",
		MemoryBufferLimit: "10M",
		StorageType:       "filesystem",
	}

	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "logpipeline1",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				Custom: `
    name stdout`,
			},
		},
	}

	expected := `[FILTER]
    name                  rewrite_tag
    match                 kube.*
    emitter_mem_buf_limit 10M
    emitter_name          logpipeline1-stdout
    emitter_storage.type  filesystem
    rule                  $log "^.*$" logpipeline1.$TAG true

`
	actual := createRewriteTagFilter(logPipeline, pipelineConfig)
	require.Equal(t, expected, actual)
}
