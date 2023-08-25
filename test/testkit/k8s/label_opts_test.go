package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	latestVersion = "1.0.0"
	recentVersion = "1.1.0"
)

func TestNil(t *testing.T) {
	var l Labels

	l.Version(latestVersion)

	assert.Len(t, l, 1)
	assert.Equal(t, l[VersionLabelName], latestVersion)
}

func TestExistingVersion(t *testing.T) {
	l := Labels{VersionLabelName: latestVersion}

	l.Version(recentVersion)

	assert.Len(t, l, 1)
	assert.Equal(t, l[VersionLabelName], recentVersion)
}

func TestExistingOther(t *testing.T) {
	l := Labels{"name-label": "name"}

	l.Version(recentVersion)

	assert.Len(t, l, 2)
	assert.Equal(t, l[VersionLabelName], recentVersion)
}
