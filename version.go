package main

// Version information injected at build time
var (
	AppName = "cw-ping-probe"
	// Version is the application's version
	Version = "dev"
	// BuildTime is the time the application was built
	BuildTime = "unknown"
	// GitCommit is the git commit hash the application was built from
	GitCommit = "unknown"
)

// GetVersionInfo returns a formatted string containing version information
func GetVersionInfo() string {
	return AppName + " " + Version + " (build: " + BuildTime + ", commit: " + GitCommit + ")"
}
