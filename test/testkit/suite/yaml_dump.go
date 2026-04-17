package suite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// dumpResourceYAML collects the YAML of all pipeline CRDs, the Telemetry CR, and
// all Deployments/DaemonSets/StatefulSets in kyma-system, then writes the result
// to dir/resources.yaml. Never fails the test.
func dumpResourceYAML(t *testing.T, ctx context.Context, dir string) {
	t.Helper()

	var buf bytes.Buffer

	collectPipelineCRDs(t, ctx, &buf)
	collectTelemetryCR(t, ctx, &buf)
	collectSystemWorkloads(t, ctx, &buf)

	content := buf.String()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Logf("artifacts dir create error (%s): %v", dir, err)
		return
	}

	path := filepath.Join(dir, "resources.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Logf("artifact write error (%s): %v", path, err)
	}
}

func collectPipelineCRDs(t *testing.T, ctx context.Context, buf *bytes.Buffer) {
	t.Helper()

	var logPipelines telemetryv1beta1.LogPipelineList
	if err := K8sClient.List(ctx, &logPipelines); err != nil {
		t.Logf("list LogPipelines error: %v", err)
	} else {
		for i := range logPipelines.Items {
			appendObjYAML(t, buf, &logPipelines.Items[i], fmt.Sprintf("LogPipeline/%s", logPipelines.Items[i].Name))
		}
	}

	var metricPipelines telemetryv1beta1.MetricPipelineList
	if err := K8sClient.List(ctx, &metricPipelines); err != nil {
		t.Logf("list MetricPipelines error: %v", err)
	} else {
		for i := range metricPipelines.Items {
			appendObjYAML(t, buf, &metricPipelines.Items[i], fmt.Sprintf("MetricPipeline/%s", metricPipelines.Items[i].Name))
		}
	}

	var tracePipelines telemetryv1beta1.TracePipelineList
	if err := K8sClient.List(ctx, &tracePipelines); err != nil {
		t.Logf("list TracePipelines error: %v", err)
	} else {
		for i := range tracePipelines.Items {
			appendObjYAML(t, buf, &tracePipelines.Items[i], fmt.Sprintf("TracePipeline/%s", tracePipelines.Items[i].Name))
		}
	}
}

func collectTelemetryCR(t *testing.T, ctx context.Context, buf *bytes.Buffer) {
	t.Helper()

	var telemetryList operatorv1beta1.TelemetryList
	if err := K8sClient.List(ctx, &telemetryList); err != nil {
		t.Logf("list Telemetry error: %v", err)
		return
	}

	for i := range telemetryList.Items {
		appendObjYAML(t, buf, &telemetryList.Items[i], fmt.Sprintf("Telemetry/%s", telemetryList.Items[i].Name))
	}
}

func collectSystemWorkloads(t *testing.T, ctx context.Context, buf *bytes.Buffer) {
	t.Helper()

	var deployments appsv1.DeploymentList
	if err := K8sClient.List(ctx, &deployments, client.InNamespace(systemNamespace)); err != nil {
		t.Logf("list Deployments error: %v", err)
	} else {
		for i := range deployments.Items {
			appendObjYAML(t, buf, &deployments.Items[i], fmt.Sprintf("Deployment/%s", deployments.Items[i].Name))
		}
	}

	var daemonSets appsv1.DaemonSetList
	if err := K8sClient.List(ctx, &daemonSets, client.InNamespace(systemNamespace)); err != nil {
		t.Logf("list DaemonSets error: %v", err)
	} else {
		for i := range daemonSets.Items {
			appendObjYAML(t, buf, &daemonSets.Items[i], fmt.Sprintf("DaemonSet/%s", daemonSets.Items[i].Name))
		}
	}

	var statefulSets appsv1.StatefulSetList
	if err := K8sClient.List(ctx, &statefulSets, client.InNamespace(systemNamespace)); err != nil {
		t.Logf("list StatefulSets error: %v", err)
	} else {
		for i := range statefulSets.Items {
			appendObjYAML(t, buf, &statefulSets.Items[i], fmt.Sprintf("StatefulSet/%s", statefulSets.Items[i].Name))
		}
	}
}

func appendObjYAML(t *testing.T, buf *bytes.Buffer, obj client.Object, header string) {
	t.Helper()

	raw, err := marshalToYAML(obj)
	if err != nil {
		t.Logf("yaml marshal error (%s): %v", header, err)
		return
	}

	fmt.Fprintf(buf, "# %s\n---\n%s\n", header, raw)
}

func marshalToYAML(obj client.Object) (string, error) {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return "", fmt.Errorf("yaml convert: %w", err)
	}

	return string(yamlBytes), nil
}
