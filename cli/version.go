package cli

import "runtime/debug"

// BuildVersion reports the running binary's real build identity — its
// VCS revision, truncated the way `git rev-parse --short` would, with a
// "+" marker if the working tree had uncommitted changes at build time —
// rather than a fabricated semantic version. Foundry has no release
// process or version scheme yet (pre-1.0, ADR-0009): claiming a "v0.x.y"
// would assert a promise this project does not make. Returns "dev" when
// no VCS revision is embedded (e.g. `go run`, or a binary built outside
// a git checkout).
func BuildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		return "dev"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified {
		return "dev (" + revision + "+)"
	}
	return "dev (" + revision + ")"
}
