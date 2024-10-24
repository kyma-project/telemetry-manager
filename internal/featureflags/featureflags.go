package featureflags

type FeatureFlag int

const (
	V1Beta1 FeatureFlag = iota
	LogPipelineOTLP
)

var f = &map[FeatureFlag]bool{}

func Enable(flag FeatureFlag) {
	Set(flag, true)
}

func Disable(flag FeatureFlag) {
	Set(flag, false)
}

func Set(flag FeatureFlag, enabled bool) {
	(*f)[flag] = enabled
}

func IsEnabled(flag FeatureFlag) bool {
	return (*f)[flag]
}

func EnabledFlags() []FeatureFlag {
	flags := []FeatureFlag{}

	for flag, enabled := range *f {
		if enabled {
			flags = append(flags, flag)
		}
	}

	return flags
}
