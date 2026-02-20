package version

// version is set at build time via:
//
//	-ldflags "-X agent-cli/internal/version.version=<semver>"
var version string

// Version returns the application version. It defaults to "dev" when
// no value has been injected at build time.
func Version() string {
	if version == "" {
		return "dev"
	}

	return version
}
