package labels

const (
	selectorLabelKey = "app.kubernetes.io/name"
)

func MakeDefaultLabel(baseName string) map[string]string {
	return map[string]string{
		selectorLabelKey: baseName,
	}
}
