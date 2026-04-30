package main

import (
	"fmt"
	"os"

	"github.com/NomiciAI/nomici-orchestrator/internal/cli"
)

var version = "dev"

func main() {
	root := cli.NewRootCommand(version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
