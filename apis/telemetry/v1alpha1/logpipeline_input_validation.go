package v1alpha1

import "fmt"

func (lp *LogPipeline) ValidateInput() error {
	input := lp.Spec.Input
	if &input == nil {
		return nil
	}

	var containers = input.Application.Containers
	if len(containers.Include) > 0 && len(containers.Exclude) > 0 {
		return fmt.Errorf("invalid log pipeline definition: can not define both 'input.application.containers.include' and 'input.application.containers.exclude'")
	}

	var namespaces = input.Application.Namespaces
	if (len(namespaces.Include) > 0 && len(namespaces.Exclude) > 0) ||
		(len(namespaces.Include) > 0 && namespaces.System) ||
		(len(namespaces.Exclude) > 0 && namespaces.System) {
		return fmt.Errorf("invalid log pipeline definition: can only define one of 'input.application.namespaces' selectors: 'include', 'exclude', 'system'")
	}

	return nil
}
