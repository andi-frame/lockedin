// Command tepatictl is the admin CLI (seed, settlement replay, checks).
package main

import (
	"fmt"
	"os"

	"github.com/andi-frame/lockedin/apps/server/internal/buildinfo"
)

const usage = `usage: tepatictl <command>

commands:
  version   print the build version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println(buildinfo.Version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}
