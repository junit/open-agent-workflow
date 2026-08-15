package cli

import (
	"fmt"
	"io"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, rootUsage())
		return 0
	}
	switch args[0] {
	case "profile":
		return runProfile(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	default:
		return runManagement(args, stdout, stderr)
	}
}

func rootUsage() string {
	return installerUsage() + "\n" + profileUsage()
}
