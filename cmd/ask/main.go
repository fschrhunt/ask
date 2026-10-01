// ask hands tasks to local coding agents and records their answers.
package main

import (
	"os"

	"github.com/fschrhunt/ask/internal/cli"
)

// version is injected by release linker flags; ordinary builds report dev.
var version = "dev"

// main delegates the CLI and exits with its contract status.
func main() { os.Exit(cli.Main(os.Args[1:], version)) }
