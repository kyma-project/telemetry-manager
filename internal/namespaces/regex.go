package namespaces

import "regexp"

// ValidNameRegexp is the regular expression used to validate Kubernetes namespace names.
var ValidNameRegexp = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
