// Command helmcov reports line and branch coverage for Helm chart templates.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/ClickHouse/helmcov/internal/cli"
)

// version is set by GoReleaser's default ldflags (-X main.version=...).
var version string

func main() {
	cli.Version = buildVersion()
	if err := cli.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// buildVersion falls back to the module version for `go install ...@vX.Y.Z` builds.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
