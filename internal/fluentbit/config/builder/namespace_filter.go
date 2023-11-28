package builder

import (
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	ns "github.com/kyma-project/telemetry-manager/internal/namespaces"
)

func createNamespaceGrepFilter(pipeline *telemetryv1alpha1.LogPipeline) string {
	namespaces := pipeline.Spec.Input.Application.Namespaces
	if namespaces.System {
		return ""
	}

	var sectionBuilder = NewFilterSectionBuilder().
		AddConfigParam("Name", "grep").
		AddConfigParam("Match", fmt.Sprintf("%s.*", pipeline.Name))

	if len(namespaces.Include) > 0 {
		return sectionBuilder.
			AddConfigParam("Regex", fmt.Sprintf("$kubernetes['namespace_name'] %s", strings.Join(namespaces.Include, "|"))).
			Build()
	}

	if len(namespaces.Exclude) > 0 {
		return sectionBuilder.
			AddConfigParam("Exclude", fmt.Sprintf("$kubernetes['namespace_name'] %s", strings.Join(namespaces.Exclude, "|"))).
			Build()
	}

	return sectionBuilder.
		AddConfigParam("Exclude", fmt.Sprintf("$kubernetes['namespace_name'] %s", strings.Join(ns.System(), "|"))).
		Build()
}

func createContainerGrepFilter(pipeline *telemetryv1alpha1.LogPipeline) string {
	containers := pipeline.Spec.Input.Application.Containers

	if len(containers.Include) == 0 && len(containers.Exclude) == 0 {
		return ""
	}

	var sectionBuilder = NewFilterSectionBuilder().
		AddConfigParam("Name", "grep").
		AddConfigParam("Match", fmt.Sprintf("%s.*", pipeline.Name))

	if len(containers.Include) > 0 {
		return sectionBuilder.
			AddConfigParam("Regex", fmt.Sprintf("$kubernetes['container_name'] %s", strings.Join(containers.Include, "|"))).
			Build()
	}

	if len(containers.Exclude) > 0 {
		return sectionBuilder.
			AddConfigParam("Exclude", fmt.Sprintf("$kubernetes['container_name'] %s", strings.Join(containers.Exclude, "|"))).
			Build()
	}

	return sectionBuilder.Build()
}
