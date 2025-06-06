package build

import "runtime/debug"

var (
	// GitTag is the gitTag of the telemetry manager.
	gitTag = "main"
	// GitCommit is the git commit hash of the telemetry manager.
	gitCommit = "unknown"
	// GitTreeState is the state of the git tree when the telemetry manager was built.
	gitTreeState = "unknown"
)

func VersionInfo() string {
	return "GitTag: " + gitTag +
		", GitCommit: " + gitCommit +
		", GitTreeState: " + gitTreeState
}

func AsLabels() map[string]string {
	return map[string]string{
		"git_tag":        gitTag,
		"git_commit":     GitCommit(),
		"go_version":     GoVersion(),
		"git_tree_state": gitTreeState,
	}
}

func GitTag() string {
	return gitTag
}

func GitCommit() string {
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

func GoVersion() string {
	buildInfo, ok := readBuildInfo()
	if !ok {
		return "unknown"
	}

	return buildInfo.GoVersion
}

func GitTreeState() string {
	return gitTreeState
}
