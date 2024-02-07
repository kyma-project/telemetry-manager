package builder

import (
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
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
	inputBuilder.AddConfigParam("mem_buf_limit", "5MB")

	return inputBuilder.Build()
}

func createIncludePath(pipeline *telemetryv1alpha1.LogPipeline) string {
	var includePath []string

	includeNamespaces := []string{"*"}
	if len(pipeline.Spec.Input.Application.Namespaces.Include) > 0 {
		includeNamespaces = pipeline.Spec.Input.Application.Namespaces.Include
	}

	includeContainers := []string{"*"}
	if len(pipeline.Spec.Input.Application.Containers.Include) > 0 {
		includeContainers = pipeline.Spec.Input.Application.Containers.Include
	}

	for _, ns := range includeNamespaces {
		for _, container := range includeContainers {
			includePath = append(includePath, makeLogPath(ns, "*", container))
		}
	}

	return strings.Join(includePath, ",")
}

func createExcludePath(pipeline *telemetryv1alpha1.LogPipeline) string {
	excludePath := []string{
		makeLogPath("kyma-system", "telemetry-fluent-bit-*", "fluent-bit"),
	}

	excludeNamespaces := pipeline.Spec.Input.Application.Namespaces.Exclude
	if !pipeline.Spec.Input.Application.Namespaces.System && len(pipeline.Spec.Input.Application.Namespaces.Include) == 0 && len(pipeline.Spec.Input.Application.Namespaces.Exclude) == 0 {
		excludeNamespaces = namespaces.System()
	}

	for _, ns := range excludeNamespaces {
		excludePath = append(excludePath, makeLogPath(ns, "*", "*"))
	}

	for _, container := range pipeline.Spec.Input.Application.Containers.Exclude {
		excludePath = append(excludePath, makeLogPath("*", "*", container))
	}

	return strings.Join(excludePath, ",")
}

func makeLogPath(namespace, pod, container string) string {
	pathPattern := "/var/log/containers/%s_%s_%s-*.log"
	return fmt.Sprintf(pathPattern, pod, namespace, container)
}
