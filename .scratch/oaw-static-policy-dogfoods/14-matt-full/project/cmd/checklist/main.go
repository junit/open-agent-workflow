package main

import (
	"fmt"
	"io"
	"os"

	"example.com/oaw-dogfood/matt-full/internal/checklist"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: checklist <markdown-path>")
		return 2
	}
	content, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "read checklist %q: %v\n", args[0], err)
		return 1
	}
	summary := checklist.Summarize(string(content))
	fmt.Fprintf(stdout, "%d/%d complete\n", summary.Complete, summary.Total)
	return 0
}
