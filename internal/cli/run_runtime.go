package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

type runtimeExchangeCommand struct {
	stateRoot string
}

type runtimeDenial struct {
	SchemaVersion string `json:"schema_version"`
	Kind          string `json:"kind"`
	Reason        string `json:"reason"`
}

func runRuntimeExchange(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	parsed, err := parseRuntimeExchange(args)
	if err != nil {
		return writeRuntimeDenial("INVALID_ARGUMENT", err, 64, stdout, stderr)
	}
	engine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: parsed.stateRoot})
	if err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	if err := oawruntime.ExchangeJSON(stdin, stdout, engine); err != nil {
		return writeRuntimeDenial(runtimeReason(err), err, 65, stdout, stderr)
	}
	return 0
}

func parseRuntimeExchange(args []string) (runtimeExchangeCommand, error) {
	if len(args) == 0 || args[0] != "exchange" {
		return runtimeExchangeCommand{}, fmt.Errorf("expected runtime exchange command")
	}
	result := runtimeExchangeCommand{stateRoot: defaultRuntimeStateRoot()}
	seen := false
	for index := 1; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--state-root":
			if seen || index+1 >= len(args) {
				return runtimeExchangeCommand{}, fmt.Errorf("--state-root requires one value")
			}
			seen = true
			result.stateRoot = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--state-root="):
			if seen {
				return runtimeExchangeCommand{}, fmt.Errorf("--state-root may be specified only once")
			}
			seen = true
			result.stateRoot = strings.TrimPrefix(argument, "--state-root=")
			index++
		default:
			return runtimeExchangeCommand{}, fmt.Errorf("unknown runtime exchange argument %q", argument)
		}
	}
	if result.stateRoot == "" || !filepath.IsAbs(result.stateRoot) || filepath.Clean(result.stateRoot) != result.stateRoot || strings.IndexFunc(result.stateRoot, unicode.IsControl) >= 0 {
		return runtimeExchangeCommand{}, fmt.Errorf("state root must be a clean absolute path")
	}
	return result, nil
}

func defaultRuntimeStateRoot() string {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return ""
		}
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "open-agent-workflow", "runtime")
}

func runtimeReason(err error) string {
	if code := oawruntime.ErrorCode(err); code != "" {
		return code
	}
	return "RUNTIME_INTERNAL"
}

func writeRuntimeDenial(reason string, err error, status int, stdout, stderr io.Writer) int {
	encoded, encodeErr := canonicaljson.Marshal(runtimeDenial{
		SchemaVersion: oawruntime.RuntimeSchemaV1,
		Kind:          "DENIED",
		Reason:        reason,
	})
	if encodeErr != nil {
		fmt.Fprintf(stderr, "oaw: RUNTIME_REPLY_ENCODE_FAILED: %v\n", encodeErr)
		return 70
	}
	if _, writeErr := stdout.Write(encoded); writeErr != nil {
		fmt.Fprintf(stderr, "oaw: RUNTIME_REPLY_WRITE_FAILED: %v\n", writeErr)
		return 74
	}
	fmt.Fprintf(stderr, "oaw: %s: %v\n", reason, err)
	return status
}
