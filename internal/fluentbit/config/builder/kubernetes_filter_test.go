package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestCreateKubernetesFilterKeepAnnotations(t *testing.T) {
	expected := `[FILTER]
    name                kubernetes
    match               test-logpipeline.*
    annotations         on
    buffer_size         1MB
    k8s-logging.exclude off
    k8s-logging.parser  on
    keep_log            on
    kube_tag_prefix     test-logpipeline.var.log.containers.
    labels              on
    merge_log           on

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{
				Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
					FluentBitKeepAnnotations: ptr.To(true),
				}}}}

	actual := createKubernetesFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestCreateKubernetesFilterDropLabels(t *testing.T) {
	expected := `[FILTER]
    name                kubernetes
    match               test-logpipeline.*
    annotations         off
    buffer_size         1MB
    k8s-logging.exclude off
    k8s-logging.parser  on
    keep_log            on
    kube_tag_prefix     test-logpipeline.var.log.containers.
    labels              off
    merge_log           on

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{
				Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
					FluentBitDropLabels: ptr.To(true),
				}}}}

	actual := createKubernetesFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestCreateKubernetesFilterKeepOriginalBodyTrue(t *testing.T) {
	expected := `[FILTER]
    name                kubernetes
    match               test-logpipeline.*
    annotations         off
    buffer_size         1MB
    k8s-logging.exclude off
    k8s-logging.parser  on
    keep_log            on
    kube_tag_prefix     test-logpipeline.var.log.containers.
    labels              on
    merge_log           on

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{
				Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
					KeepOriginalBody: ptr.To(true),
				}}}}

	actual := createKubernetesFilter(logPipeline)
	require.Equal(t, expected, actual)
}

func TestCreateKubernetesFilterKeepOriginalBodyFalse(t *testing.T) {
	expected := `[FILTER]
    name                kubernetes
    match               test-logpipeline.*
    annotations         off
    buffer_size         1MB
    k8s-logging.exclude off
    k8s-logging.parser  on
    keep_log            off
    kube_tag_prefix     test-logpipeline.var.log.containers.
    labels              on
    merge_log           on

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{
				Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
					KeepOriginalBody: ptr.To(false),
				}}}}

	actual := createKubernetesFilter(logPipeline)
	require.Equal(t, expected, actual)
}
