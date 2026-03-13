package builder

import (
	"fmt"
	"strings"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

func createInputSection(pipeline *telemetryv1beta1.LogPipeline, includePath, excludePath string) string {
	inputBuilder := NewInputSectionBuilder()
	inputBuilder.AddConfigParam("name", "tail")
	inputBuilder.AddConfigParam("alias", pipeline.Name)
	inputBuilder.AddConfigParam("path", includePath)
	inputBuilder.AddIfNotEmpty("exclude_path", excludePath)
	inputBuilder.AddConfigParam("multiline.parser", "cri")
	inputBuilder.AddConfigParam("tag", fmt.Sprintf("%s.*", pipeline.Name))
	inputBuilder.AddConfigParam("skip_long_lines", "on")
	inputBuilder.AddConfigParam("db", fmt.Sprintf("/data/flb_%s.db", pipeline.Name))
	inputBuilder.AddConfigParam("storage.type", "filesystem")
	inputBuilder.AddConfigParam("read_from_head", "true")
	inputBuilder.AddConfigParam("mem_buf_limit", "5MB")

	return inputBuilder.Build()
}

func createIncludePath(pipeline *telemetryv1beta1.LogPipeline) string {
	var includePath []string

	includeNamespaces := []string{"*"}
	includeContainers := []string{"*"}

	if pipeline.Spec.Input.Runtime != nil {
		if pipeline.Spec.Input.Runtime.Namespaces != nil && len(pipeline.Spec.Input.Runtime.Namespaces.Include) > 0 {
			includeNamespaces = pipeline.Spec.Input.Runtime.Namespaces.Include
		}

		if pipeline.Spec.Input.Runtime.Containers != nil && len(pipeline.Spec.Input.Runtime.Containers.Include) > 0 {
			includeContainers = pipeline.Spec.Input.Runtime.Containers.Include
		}
	}

	for _, ns := range includeNamespaces {
		for _, container := range includeContainers {
			includePath = append(includePath, makeLogPath(ns, "*", container))
		}
	}

	return strings.Join(includePath, ",")
}

func createExcludePath(pipeline *telemetryv1beta1.LogPipeline, collectAgentLogs bool) string {
	var excludePath []string
	if !collectAgentLogs {
		excludePath = append(excludePath, makeLogPath("kyma-system", fmt.Sprintf("%s-*", names.FluentBit), "fluent-bit"))
	}

	excludeSytemLogAgentPath := makeLogPath("kyma-system", fmt.Sprintf("*%s-*", commonresources.SystemLogAgentName), "collector")
	excludeSytemLogCollectorPath := makeLogPath("kyma-system", fmt.Sprintf("*%s-*", commonresources.SystemLogCollectorName), "collector")
	excludeOtlpLogAgentPath := makeLogPath("kyma-system", fmt.Sprintf("%s-*", names.LogAgent), "collector")

	excludePath = append(excludePath, excludeSytemLogAgentPath, excludeSytemLogCollectorPath, excludeOtlpLogAgentPath)

	var excludeNamespaces []string

	if pipeline.Spec.Input.Runtime != nil && pipeline.Spec.Input.Runtime.Namespaces != nil {
		excludeNamespaces = pipeline.Spec.Input.Runtime.Namespaces.Exclude
	} else {
		excludeNamespaces = namespaces.System()
	}

	for _, ns := range excludeNamespaces {
		excludePath = append(excludePath, makeLogPath(ns, "*", "*"))
	}

	if pipeline.Spec.Input.Runtime != nil && pipeline.Spec.Input.Runtime.Containers != nil {
		for _, container := range pipeline.Spec.Input.Runtime.Containers.Exclude {
			excludePath = append(excludePath, makeLogPath("*", "*", container))
		}
	}

	return strings.Join(excludePath, ",")
}

func makeLogPath(namespace, pod, container string) string {
	pathPattern := "/var/log/containers/%s_%s_%s-*.log"
	return fmt.Sprintf(pathPattern, pod, namespace, container)
}
