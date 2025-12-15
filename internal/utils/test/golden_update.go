//go:build updateGolden

package test

// ShouldUpdateGoldenFiles returns true when the updateGolden build tag is set.
// This file is compiled only when running: go test -tags=updateGolden
func ShouldUpdateGoldenFiles() bool {
	return true
}
