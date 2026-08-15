// Command winforge is the WinForge entrypoint: a self-contained Windows tuning
// and maintenance toolkit exposed as a CLI and a local web dashboard.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"winforge/internal/cli"
)

func main() {
	err := cli.Run(os.Args[1:])
	switch {
	case err == nil:
		return
	case errors.Is(err, flag.ErrHelp):
		// "winforge apply -h" asked for usage and the flag package already
		// printed it. Asking for help is not a failure, so exit cleanly rather
		// than reporting "flag: help requested" as an error.
		return
	default:
		fmt.Fprintln(os.Stderr, "winforge:", err)
		os.Exit(1)
	}
}
