package main

import (
	"fmt"
	"os"

	"github.com/wifibaby4u/open-agent-workflow/internal/dogfood"
)

func main() {
	if err := dogfood.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "dogfood-current: error: %v\n", err)
		os.Exit(1)
	}
}
