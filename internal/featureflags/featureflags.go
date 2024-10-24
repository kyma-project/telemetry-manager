package featureflags

var f = &flags{
	v1beta1Enabled:         false,
	logPipelineOTLPEnabled: false,
}

type flags struct {
	v1beta1Enabled         bool
	logPipelineOTLPEnabled bool
}

func Setv1Beta1Enabled(enabled bool) {
	f.v1beta1Enabled = enabled
}

func Isv1Beta1Enabled() bool {
	return f.v1beta1Enabled
}

func SetLogPipelineOTLPEnabled(enabled bool) {
	f.logPipelineOTLPEnabled = enabled
}

func IsLogPipelineOTLPEnabled() bool {
	return f.logPipelineOTLPEnabled
}
