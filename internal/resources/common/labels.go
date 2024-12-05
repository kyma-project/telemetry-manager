package common

const (
	nameLabelKey = "app.kubernetes.io/name"
)

func MakeDefaultLabels(baseName string) map[string]string {
	return map[string]string{
		nameLabelKey: baseName,
	}
}
