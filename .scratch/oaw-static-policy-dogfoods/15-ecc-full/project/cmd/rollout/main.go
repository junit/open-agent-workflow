package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"example.com/rollout/internal/rollout"
)

const usage = "usage: rollout <percentage> <key> [key...]"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	percentage, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "invalid percentage %q\n", args[0])
		return 2
	}
	selected, err := rollout.Select(percentage, args[1:])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, key := range selected {
		fmt.Fprintln(stdout, key)
	}
	return 0
}
