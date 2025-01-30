package common

const (
	nameLabelKey     = "app.kubernetes.io/name"
	moduleLabelKey   = "kyma-project.io/module"
	moduleLabelValue = "telemetry"
	partOfLabelKey   = "app.kubernetes.io/part-of"
	partOfLabelValue = "telemetry"
)

func MakeDefaultLabels(baseName string) map[string]string {
	return map[string]string{
		nameLabelKey:   baseName,
		moduleLabelKey: moduleLabelValue,
		partOfLabelKey: partOfLabelValue,
	}
}

func MakeDefaultSelectorLabels(baseName string) map[string]string {
	return map[string]string{
		nameLabelKey: baseName,
	}
}
