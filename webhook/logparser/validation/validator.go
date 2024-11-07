package validation

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
)

func Validate(lp *telemetryv1alpha1.LogParser) error {
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
