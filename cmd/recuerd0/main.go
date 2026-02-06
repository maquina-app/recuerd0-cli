package main

import (
	"runtime/debug"

	"github.com/maquina/recuerd0-cli/internal/commands"
)

// version is set via ldflags at build time: -X main.version=v1.0.0
var version string

func main() {
	v := version
	if v == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			v = info.Main.Version
		} else {
			v = "dev"
		}
	}
	commands.SetVersion(v)
	commands.Execute()
}
