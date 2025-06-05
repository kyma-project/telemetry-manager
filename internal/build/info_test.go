package build

import (
	"reflect"
	"runtime/debug"
	"testing"
)

func TestReturnsCorrectVersionInfo(t *testing.T) {
	expected := "GitTag: main, GitCommit: unknown, GitTreeState: unknown"
	actual := VersionInfo()

	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestReturnsCorrectGitTag(t *testing.T) {
	expected := "main"
	actual := GitTag()

	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestReturnsShortenedGitCommit(t *testing.T) {
	gitCommit = "123456789abcdef"
	expected := "1234567"
	actual := GitCommit()

	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestReturnsUnknownGitCommitWhenEmpty(t *testing.T) {
	gitCommit = ""
	expected := "unknown"
	actual := GitCommit()

	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestReturnsCorrectGoVersion(t *testing.T) {
	buildInfo := &debug.BuildInfo{GoVersion: "go1.20"}
	readBuildInfo = func() (*debug.BuildInfo, bool) { return buildInfo, true }
	expected := "go1.20"
	actual := GoVersion()

	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestReturnsUnknownGoVersionWhenUnavailable(t *testing.T) {
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	expected := "unknown"
	actual := GoVersion()

	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestReturnsCorrectGitTreeState(t *testing.T) {
	expected := "unknown"
	actual := GitTreeState()

	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestReturnsCorrectLabels(t *testing.T) {
	buildInfo := &debug.BuildInfo{GoVersion: "go1.20"}
	readBuildInfo = func() (*debug.BuildInfo, bool) { return buildInfo, true }
	gitTag = "main"
	gitCommit = "123456789abcdef"
	gitTreeState = "clean"
	expected := map[string]string{
		"git_tag":        "main",
		"git_commit":     "1234567",
		"go_version":     "go1.20",
		"git_tree_state": "clean",
	}
	actual := AsLabels()

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}
