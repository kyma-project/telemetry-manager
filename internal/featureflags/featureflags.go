package featureflags

type FeatureFlag int

const (
	// keeping the code with a placeholder feature flag to make introducing feature flags in the future easier
	placeholder       FeatureFlag = iota // placeholder feature flag for testing purposes and make sure the codegen works correctly
	DeployOTLPGateway FeatureFlag = iota
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
