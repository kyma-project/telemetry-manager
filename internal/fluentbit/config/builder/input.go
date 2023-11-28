package builder

import (
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func createInputSection(pipeline *telemetryv1alpha1.LogPipeline, includePath, excludePath string) string {
	inputBuilder := NewInputSectionBuilder()
	inputBuilder.AddConfigParam("name", "tail")
	inputBuilder.AddConfigParam("alias", pipeline.Name)
	inputBuilder.AddConfigParam("path", includePath)
	inputBuilder.AddConfigParam("exclude_path", excludePath)
	inputBuilder.AddConfigParam("multiline.parser", "docker, cri, go, python, java")
	inputBuilder.AddConfigParam("tag", fmt.Sprintf("%s.*", pipeline.Name))
	inputBuilder.AddConfigParam("skip_long_lines", "on")
	inputBuilder.AddConfigParam("db", fmt.Sprintf("/data/flb_%s.db", pipeline.Name))
	inputBuilder.AddConfigParam("storage.type", "filesystem")
	inputBuilder.AddConfigParam("read_from_head", "true")

	return inputBuilder.Build()
}

func createIncludePath(_ *telemetryv1alpha1.LogPipeline) string {
	var toInclude []string

	// TODO: add additional paths from pipeline spec

	if len(toInclude) == 0 {
		return makeLogPath("*", "*", "*")
	}
	return strings.Join(toInclude, ",")
}

func createExcludePath(_ *telemetryv1alpha1.LogPipeline) string {
	toExclude := []string{
		makeLogPath("kyma-system", "telemetry-fluent-bit-*", "fluent-bit"),
	}

	// TODO: add additional paths from pipeline spec

	return strings.Join(toExclude, ",")
}

func makeLogPath(namespace, pod, container string) string {
	pathPattern := "/var/log/containers/%s_%s_%s-*.log"
	return fmt.Sprintf(pathPattern, pod, namespace, container)
}
