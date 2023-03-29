package v1alpha1

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
)

func (lp *LogParser) Validate() error {
	if len(lp.Spec.Parser) == 0 {
		return fmt.Errorf("log parser '%s' has no parser defined", lp.Name)
	}
	section, err := config.ParseCustomSection(lp.Spec.Parser)
	if err != nil {
		return err
	}
	if section.ContainsKey("name") {
		return fmt.Errorf("log parser '%s' cannot have name defined in parser section", lp.Name)
	}
	return nil
}
