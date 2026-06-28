package version

import "runtime/debug"

var Version = "dev"

func String() string {
	return resolve(Version, build())
}

func resolve(explicit, build string) string {
	if explicit != "" && explicit != "dev" && explicit != "(devel)" {
		return explicit
	}
	if build != "" && build != "(devel)" {
		return build
	}
	return "dev"
}

func build() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		return bi.Main.Version
	}
	return ""
}
