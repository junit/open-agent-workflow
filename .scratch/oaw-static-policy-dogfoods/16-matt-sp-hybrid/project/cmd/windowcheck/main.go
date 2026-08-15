package main

import (
	"fmt"
	"io"
	"os"

	"example.com/windowcheck/internal/maintenance"
)

const usage = "usage: windowcheck <window> [window...]"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	summary, err := maintenance.Evaluate(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, summary)
	return 0
}
