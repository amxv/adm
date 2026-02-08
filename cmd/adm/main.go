package main

import "github.com/amxv/adm/internal/cli"

// Set via -ldflags at build time.
var version = "dev"

func main() {
	cli.SetVersion(version)
	cli.Execute()
}
