// build.sh will generate a gen.go during the build process.
// This file is only used when directly running source code.
package buildinfo

const (
	Version string = "source"
	Mode    string = "debug"
	Time    string = "unknown"
)
