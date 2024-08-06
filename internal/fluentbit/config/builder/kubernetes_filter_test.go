package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					KeepAnnotations: true,
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
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					DropLabels: true,
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
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
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
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					KeepOriginalBody: ptr.To(false),
				}}}}

	actual := createKubernetesFilter(logPipeline)
	require.Equal(t, expected, actual)
}
