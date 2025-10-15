package build

import (
	"crypto/fips140"
	"runtime/debug"
	"strconv"
)

var (
	// GitTag is the gitTag of the telemetry manager.
	gitTag = "main"
	// GitCommit is the git commit hash of the telemetry manager.
	gitCommit = "unknown"
	// GitTreeState is the state of the git tree when the telemetry manager was built.
	gitTreeState = "unknown"
)

func InfoMap() map[string]string {
	return map[string]string{
		"git_tag":           gitTag,
		"git_commit":        shortenedGitCommit(),
		"go_version":        goVersion(),
		"git_tree_state":    gitTreeState,
		"fips_mode_enabled": strconv.FormatBool(fips140.Enabled()),
	}
}

func GitTag() string {
	return gitTag
}

func shortenedGitCommit() string {
	if gitCommit == "" {
		return "unknown"
	}

	const shortSHALength = 7
	if len(gitCommit) > shortSHALength {
		return gitCommit[:shortSHALength]
	}

	return gitCommit
}

var readBuildInfo = debug.ReadBuildInfo

func goVersion() string {
	buildInfo, ok := readBuildInfo()
	if !ok {
		return "unknown"
	}

	return buildInfo.GoVersion
}
