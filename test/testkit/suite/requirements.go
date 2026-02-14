package suite

import "github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"

// InferRequirementsFromLabels derives cluster configuration from test labels.
// This function maps test labels to cluster requirements, enabling dynamic
// cluster reconfiguration based on what each test needs.
//
// Label Mappings:
//   - LabelIstio: Requires Istio service mesh installation
//   - LabelExperimental: Requires experimental features enabled
//
// Future Extensions:
//   - LabelFIPS: Requires FIPS mode operation
//   - LabelCustomLabelAnnotation: Requires custom labels/annotations support
//
// The returned Config contains all requirements derived from the labels.
// Fields not influenced by labels (like ManagerImage) are left at their
// default values and should be populated by the caller.
func InferRequirementsFromLabels(labels []string) kubeprep.Config {
	cfg := kubeprep.Config{
		// Defaults - most common case for tests
		InstallIstio:            false,
		OperateInFIPSMode:       false,
		EnableExperimental:      false,
		CustomLabelsAnnotations: false,
		SkipManagerDeployment:   false,
		SkipPrerequisites:       false,
	}

	// Scan labels and enable features as needed
	for _, label := range labels {
		switch label {
		case LabelIstio:
			cfg.InstallIstio = true
		case LabelExperimental:
			cfg.EnableExperimental = true
		// Future label mappings:
		// case LabelFIPS:
		//     cfg.OperateInFIPSMode = true
		// case LabelCustomLabelAnnotation:
		//     cfg.CustomLabelsAnnotations = true
		}
	}

	return cfg
}
