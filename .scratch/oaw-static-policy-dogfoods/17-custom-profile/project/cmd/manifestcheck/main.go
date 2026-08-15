package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"example.com/manifestcheck/internal/manifest"
)

const usage = "usage: manifestcheck [--require key]... <path>"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	required := []string{"version", "commit"}
	path := ""
	for index := 0; index < len(args); {
		switch args[index] {
		case "--require":
			if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" {
				fmt.Fprintln(stderr, usage)
				return 2
			}
			required = append(required, args[index+1])
			index += 2
		default:
			if strings.HasPrefix(args[index], "--") || path != "" {
				fmt.Fprintln(stderr, usage)
				return 2
			}
			path = args[index]
			index++
		}
	}
	if path == "" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "read manifest %q: %v\n", path, err)
		return 1
	}
	summary, err := manifest.ValidateRequired(data, required)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, summary)
	return 0
}
