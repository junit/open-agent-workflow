package main

import (
	"os"

	"github.com/wifibaby4u/open-agent-workflow/internal/bridgecli"
)

func main() {
	os.Exit(bridgecli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
