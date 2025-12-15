//go:build !updateGolden

package test

// ShouldUpdateGoldenFiles returns false during normal test runs.
// This file is compiled when the updateGolden build tag is NOT set.
func ShouldUpdateGoldenFiles() bool {
	return false
}
