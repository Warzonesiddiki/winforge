// Command winforge is the WinForge entrypoint: a self-contained Windows tuning
// and maintenance toolkit exposed as a CLI and a local web dashboard.
package main

import (
	"fmt"
	"os"

	"winforge/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "winforge:", err)
		os.Exit(1)
	}
}
