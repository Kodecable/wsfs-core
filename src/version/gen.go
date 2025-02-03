// build.sh will generate a gen.go during the build process.
// This file is only used when directly running source code.
package version

const (
	Version string = "source"
	Mode    string = "debug"
	Time    string = "unknown"
)
