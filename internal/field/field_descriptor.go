package field

type SecretKeyRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Key       string `json:"key,omitempty"`
}

type Descriptor struct {
	TargetSecretKey string
	SecretKeyRef    SecretKeyRef
}
