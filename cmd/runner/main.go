package main

import (
	"herdworkbench/internal/runner"
	"os"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--system-install" {
		os.Exit(runner.RunSystemInstallerHost(os.Args[2]))
	}
	os.Exit(runner.RunHost())
}
