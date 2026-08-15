package main

import (
	"os"

	"github.com/wifibaby4u/open-agent-workflow/internal/assurancecli"
)

func main() {
	os.Exit(assurancecli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
