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

	return inputBuilder.Build()
}

func createIncludePath(pipeline *telemetryv1alpha1.LogPipeline) string {
	var toInclude []string

	if len(pipeline.Spec.Input.Application.Namespaces.Include) == 0 {
		pipeline.Spec.Input.Application.Namespaces.Include = append(pipeline.Spec.Input.Application.Namespaces.Include, "*")
	}

	if len(pipeline.Spec.Input.Application.Containers.Include) == 0 {
		pipeline.Spec.Input.Application.Containers.Include = append(pipeline.Spec.Input.Application.Containers.Include, "*")
	}

	for _, ns := range pipeline.Spec.Input.Application.Namespaces.Include {
		for _, container := range pipeline.Spec.Input.Application.Containers.Include {
			toInclude = append(toInclude, makeLogPath(ns, "*", container))
		}
	}

	return strings.Join(toInclude, ",")
}

func createExcludePath(pipeline *telemetryv1alpha1.LogPipeline) string {
	toExclude := []string{
		makeLogPath("kyma-system", "telemetry-fluent-bit-*", "fluent-bit"),
	}

	if !pipeline.Spec.Input.Application.Namespaces.System {
		pipeline.Spec.Input.Application.Namespaces.Exclude = append(pipeline.Spec.Input.Application.Namespaces.Exclude, namespaces.System()...)
	}

	for _, ns := range pipeline.Spec.Input.Application.Namespaces.Exclude {
		toExclude = append(toExclude, makeLogPath(ns, "*", "*"))
	}

	for _, container := range pipeline.Spec.Input.Application.Containers.Exclude {
		toExclude = append(toExclude, makeLogPath("*", "*", container))
	}

	return strings.Join(toExclude, ",")
}

func makeLogPath(namespace, pod, container string) string {
	pathPattern := "/var/log/containers/%s_%s_%s-*.log"
	return fmt.Sprintf(pathPattern, pod, namespace, container)
}
