package version

// version is set at build time via ldflags:
//
//	-ldflags "-X agent-cli/internal/version.version=v1.2.3"
var version = "dev" //nolint:gochecknoglobals // linker-injected build metadata

// Info returns the current build version string.
func Info() string {
	return version
}
