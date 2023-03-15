package v1alpha1

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
	"strings"
)

func (l *LogPipeline) ValidateFilters(deniedFilterPlugins []string) error {
	for _, filterPlugin := range l.Spec.Filters {
		if err := validateCustomFilter(filterPlugin.Custom, deniedFilterPlugins); err != nil {
			return err
		}
	}
	return nil
}

func validateCustomFilter(content string, deniedFilterPlugins []string) error {
	if content == "" {
		return nil
	}

	section, err := config.ParseCustomSection(content)
	if err != nil {
		return err
	}

	if !section.ContainsKey("name") {
		return fmt.Errorf("configuration section does not have name attribute")
	}

	pluginName := section.GetByKey("name").Value

	for _, deniedPlugin := range deniedFilterPlugins {
		if strings.EqualFold(pluginName, deniedPlugin) {
			return fmt.Errorf("filter plugin '%s' is forbidden. ", pluginName)
		}
	}

	if section.ContainsKey("match") {
		return fmt.Errorf("plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	return nil
}
