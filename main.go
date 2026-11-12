// Command shipo cuts a release from any repo: it reads the version, bumps it,
// builds, and publishes.
package main

import (
	"os"

	"github.com/diphyx/shipo/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=X.Y.Z".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
