// Package assurancecli implements the standalone oaw-assurance command.
package assurancecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
)

const maximumInputBytes = 4 << 20

type Environment struct {
	WorkingDir string
	Home       string
	ConfigHome string
}

type VerificationResult struct {
	SchemaVersion string                     `json:"schema_version"`
	Valid         bool                       `json:"valid"`
	Profile       assurance.ProfileReference `json:"profile"`
	OverlayDigest string                     `json:"overlay_digest"`
}

type command struct {
	action  string
	profile string
	input   string
	help    bool
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	workingDirectory, _ := os.Getwd()
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" && home != "" {
		configHome = filepath.Join(home, ".config")
	}
	return RunWithEnvironment(args, stdin, stdout, stderr, Environment{
		WorkingDir: workingDirectory, Home: home, ConfigHome: configHome,
	})
}

func RunWithEnvironment(
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	environment Environment,
) int {
	parsed, err := parseCommand(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw-assurance: INVALID_ARGUMENT: %s\n%s", err, usage())
		return 64
	}
	if parsed.help {
		fmt.Fprint(stdout, usage())
		return 0
	}
	inventory, err := profileinspect.Discover(profileinspect.Environment{
		WorkingDir: environment.WorkingDir, Home: environment.Home, ConfigHome: environment.ConfigHome,
	})
	if err != nil {
		return writeFailure(stderr, "PROFILE_INSPECTION_FAILED", err)
	}
	profile, err := profileinspect.Resolve(inventory, parsed.profile)
	if err != nil {
		return writeFailure(stderr, "PROFILE_SELECTION_INVALID", err)
	}
	if parsed.action == "inspect" {
		index, err := assurance.Inspect(profile)
		if err != nil {
			return writeAssuranceFailure(stderr, err)
		}
		return writeJSON(stdout, stderr, index)
	}
	raw, err := readInput(parsed.input, stdin)
	if err != nil {
		return writeFailure(stderr, "ASSURANCE_INPUT_INVALID", err)
	}
	switch parsed.action {
	case "issue":
		request, err := assurance.DecodeIssueRequest(raw)
		if err != nil {
			return writeAssuranceFailure(stderr, err)
		}
		overlay, err := assurance.Issue(profile, request)
		if err != nil {
			return writeAssuranceFailure(stderr, err)
		}
		return writeJSON(stdout, stderr, overlay)
	case "verify":
		overlay, err := assurance.DecodeOverlay(raw)
		if err != nil {
			return writeAssuranceFailure(stderr, err)
		}
		if err := assurance.Verify(profile, overlay); err != nil {
			return writeAssuranceFailure(stderr, err)
		}
		return writeJSON(stdout, stderr, VerificationResult{
			SchemaVersion: "oaw.assurance-verification/v1", Valid: true,
			Profile: overlay.Profile, OverlayDigest: overlay.Digest,
		})
	default:
		return writeFailure(stderr, "INVALID_ARGUMENT", fmt.Errorf("unknown overlay action %q", parsed.action))
	}
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		return command{help: true}, nil
	}
	if len(args) < 2 || args[0] != "overlay" || args[1] != "inspect" && args[1] != "issue" && args[1] != "verify" {
		return command{}, errors.New("expected overlay inspect, overlay issue, or overlay verify")
	}
	result := command{action: args[1], input: "-"}
	profileSeen := false
	inputSeen := false
	for index := 2; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-h" || arg == "--help":
			if len(args) != 3 {
				return command{}, errors.New("help cannot be combined with other arguments")
			}
			return command{help: true}, nil
		case arg == "--profile":
			if profileSeen || index+1 >= len(args) {
				return command{}, errors.New("--profile requires one value and may appear once")
			}
			profileSeen = true
			index++
			result.profile = args[index]
		case strings.HasPrefix(arg, "--profile="):
			if profileSeen {
				return command{}, errors.New("--profile may appear once")
			}
			profileSeen = true
			result.profile = strings.TrimPrefix(arg, "--profile=")
		case arg == "--input":
			if inputSeen || index+1 >= len(args) {
				return command{}, errors.New("--input requires one value and may appear once")
			}
			inputSeen = true
			index++
			result.input = args[index]
		case strings.HasPrefix(arg, "--input="):
			if inputSeen {
				return command{}, errors.New("--input may appear once")
			}
			inputSeen = true
			result.input = strings.TrimPrefix(arg, "--input=")
		default:
			return command{}, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if !profileSeen || strings.TrimSpace(result.profile) == "" {
		return command{}, errors.New("--profile is required")
	}
	if result.input == "" {
		return command{}, errors.New("--input cannot be empty")
	}
	if result.action == "inspect" && inputSeen {
		return command{}, errors.New("overlay inspect does not accept --input")
	}
	return result, nil
}

func readInput(inputPath string, stdin io.Reader) ([]byte, error) {
	if inputPath == "-" {
		return readLimited(stdin)
	}
	before, err := os.Lstat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("inspect input file: %w", err)
	}
	if !before.Mode().IsRegular() || before.Size() > maximumInputBytes {
		return nil, errors.New("input must be a regular file within the size limit")
	}
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open input file: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, errors.New("input file changed before it was read")
	}
	raw, err := readLimited(file)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	current, currentErr := os.Lstat(inputPath)
	if err != nil || currentErr != nil || !os.SameFile(opened, after) || !os.SameFile(after, current) ||
		before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.New("input file changed while it was read")
	}
	return raw, nil
}

func readLimited(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Assurance input: %w", err)
	}
	if len(raw) > maximumInputBytes {
		return nil, errors.New("Assurance input exceeds the size limit")
	}
	return raw, nil
}

func writeJSON(stdout, stderr io.Writer, value any) int {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return writeFailure(stderr, "ASSURANCE_OUTPUT_INVALID", err)
	}
	if _, err := io.Copy(stdout, &encoded); err != nil {
		return writeFailure(stderr, "ASSURANCE_OUTPUT_FAILED", err)
	}
	return 0
}

func writeAssuranceFailure(stderr io.Writer, err error) int {
	if assurance.ErrorCode(err) != "" {
		fmt.Fprintf(stderr, "oaw-assurance: %s\n", err)
		return 65
	}
	return writeFailure(stderr, "ASSURANCE_FAILED", err)
}

func writeFailure(stderr io.Writer, code string, err error) int {
	fmt.Fprintf(stderr, "oaw-assurance: %s: %s\n", code, err)
	return 65
}

func usage() string {
	return "Usage: oaw-assurance overlay inspect --profile PROFILE\n" +
		"       oaw-assurance overlay issue --profile PROFILE [--input FILE|-]\n" +
		"       oaw-assurance overlay verify --profile PROFILE [--input FILE|-]\n\n" +
		"Issues or verifies optional Profile-bound machine claims. It does not select or run a Policy Profile.\n"
}
