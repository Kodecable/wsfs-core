package version

import "runtime/debug"

var (
	Buildinfo   *debug.BuildInfo
	BuildinfoOk bool
)

func init() {
	Buildinfo, BuildinfoOk = debug.ReadBuildInfo()
}

const ReadBuildInfoFailed = "Unknown (Error: ReadBuildInfo() failed)"

func GoVersion() string {
	if BuildinfoOk {
		return Buildinfo.GoVersion
	}
	return ReadBuildInfoFailed
}

func BuildDeps() []*debug.Module {
	if BuildinfoOk {
		return Buildinfo.Deps
	}
	return []*debug.Module{}
}

func BuildSettings() []debug.BuildSetting {
	if BuildinfoOk {
		return Buildinfo.Settings
	}
	return []debug.BuildSetting{}
}
