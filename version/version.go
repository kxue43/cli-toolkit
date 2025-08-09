package version

import (
	"fmt"
	"runtime/debug"
)

func FromBuildInfo() (version string) {
	version = "unavailable"

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	var vcs, revision, ts string

	for i := range info.Settings {
		switch info.Settings[i].Key {
		case "vcs":
			vcs = info.Settings[i].Value
		case "vcs.revision":
			revision = info.Settings[i].Value
		case "vcs.time":
			ts = info.Settings[i].Value
		default:
			continue
		}
	}

	if revision == "" {
		return version
	}

	if ts == "" {
		return fmt.Sprintf("built from %s revision %s", vcs, revision)
	}

	return fmt.Sprintf("built from %s revision %s at %s", vcs, revision, ts)
}
