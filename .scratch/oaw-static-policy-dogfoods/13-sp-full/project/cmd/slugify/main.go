package main

import (
	"fmt"
	"io"
	"os"

	"example.com/oaw-dogfood/sp-full/internal/slug"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: slugify <text>")
		return 2
	}
	fmt.Fprintln(stdout, slug.Slugify(args[0]))
	return 0
}
