package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"slices"
)

const maximumCodexOutputBytes = 1 << 20

type CLIResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type CodexRunner interface {
	Run(context.Context, ...string) (CLIResult, error)
}

type ExecRunner struct {
	Binary      string
	Environment []string
	Dir         string
}

func (r ExecRunner) Run(ctx context.Context, args ...string) (CLIResult, error) {
	if !validExecutableCoordinate(r.Binary) {
		return CLIResult{ExitCode: -1}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex executable coordinate is invalid", nil)
	}
	if r.Dir != "" && !isAbsoluteCleanPath(r.Dir) {
		return CLIResult{ExitCode: -1}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex working directory is invalid", nil)
	}
	for _, argument := range args {
		if argument == "" || hasControl(argument) {
			return CLIResult{ExitCode: -1}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex argument is invalid", nil)
		}
	}

	command := exec.CommandContext(ctx, r.Binary, args...)
	command.Dir = r.Dir
	if r.Environment == nil {
		command.Env = os.Environ()
	} else {
		command.Env = slices.Clone(r.Environment)
	}
	stdout := &cappedBuffer{limit: maximumCodexOutputBytes}
	stderr := &cappedBuffer{limit: maximumCodexOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	return CLIResult{
		Stdout:   bytes.Clone(stdout.Bytes()),
		Stderr:   bytes.Clone(stderr.Bytes()),
		ExitCode: processExitCode(err),
	}, err
}

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *cappedBuffer) Write(content []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		_, _ = b.buffer.Write(content[:min(remaining, len(content))])
	}
	return len(content), nil
}

func (b *cappedBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

type PluginRecord struct {
	PluginID        string
	Name            string
	MarketplaceName string
	Version         string
	Installed       bool
	Enabled         bool
}

type MarketplaceRecord struct {
	Name       string
	Root       string
	SourceType string
	Source     string
}

func AddMarketplace(ctx context.Context, runner CodexRunner, marketplacePath string) (CLIResult, error) {
	if !isAbsoluteCleanPath(marketplacePath) {
		return CLIResult{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "marketplace path must be absolute and clean", nil)
	}
	return runCodex(ctx, runner, "plugin", "marketplace", "add", marketplacePath, "--json")
}

func AddPlugin(ctx context.Context, runner CodexRunner) (CLIResult, error) {
	return runCodex(ctx, runner, "plugin", "add", PluginName+"@"+MarketplaceName, "--json")
}

func RemovePlugin(ctx context.Context, runner CodexRunner) (CLIResult, error) {
	return runCodex(ctx, runner, "plugin", "remove", PluginName+"@"+MarketplaceName, "--json")
}

func RemoveMarketplace(ctx context.Context, runner CodexRunner) (CLIResult, error) {
	return runCodex(ctx, runner, "plugin", "marketplace", "remove", MarketplaceName, "--json")
}

func ListPlugins(ctx context.Context, runner CodexRunner) ([]PluginRecord, error) {
	result, err := runCodex(ctx, runner, "plugin", "list", "--json")
	if err != nil {
		return nil, err
	}
	var document struct {
		Installed []struct {
			PluginID        string `json:"pluginId"`
			Name            string `json:"name"`
			MarketplaceName string `json:"marketplaceName"`
			Version         string `json:"version"`
			Installed       bool   `json:"installed"`
			Enabled         bool   `json:"enabled"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(result.Stdout, &document); err != nil {
		return nil, invalidCodexOutput("decode Codex Plugin inventory", err)
	}
	records := make([]PluginRecord, 0, len(document.Installed))
	for _, item := range document.Installed {
		if item.PluginID == "" || item.Name == "" || item.MarketplaceName == "" || item.Version == "" ||
			hasControl(item.PluginID) || hasControl(item.Name) || hasControl(item.MarketplaceName) || hasControl(item.Version) {
			return nil, invalidCodexOutput("Codex Plugin inventory contains an invalid record", nil)
		}
		records = append(records, PluginRecord{
			PluginID: item.PluginID, Name: item.Name, MarketplaceName: item.MarketplaceName,
			Version: item.Version, Installed: item.Installed, Enabled: item.Enabled,
		})
	}
	return records, nil
}

func ListMarketplaces(ctx context.Context, runner CodexRunner) ([]MarketplaceRecord, error) {
	result, err := runCodex(ctx, runner, "plugin", "marketplace", "list", "--json")
	if err != nil {
		return nil, err
	}
	var document struct {
		Marketplaces []struct {
			Name              string `json:"name"`
			Root              string `json:"root"`
			MarketplaceSource struct {
				SourceType string `json:"sourceType"`
				Source     string `json:"source"`
			} `json:"marketplaceSource"`
		} `json:"marketplaces"`
	}
	if err := json.Unmarshal(result.Stdout, &document); err != nil {
		return nil, invalidCodexOutput("decode Codex marketplace inventory", err)
	}
	records := make([]MarketplaceRecord, 0, len(document.Marketplaces))
	for _, item := range document.Marketplaces {
		if item.Name == "" || hasControl(item.Name) || hasControl(item.Root) || hasControl(item.MarketplaceSource.SourceType) || hasControl(item.MarketplaceSource.Source) {
			return nil, invalidCodexOutput("Codex marketplace inventory contains an invalid record", nil)
		}
		records = append(records, MarketplaceRecord{
			Name: item.Name, Root: item.Root,
			SourceType: item.MarketplaceSource.SourceType, Source: item.MarketplaceSource.Source,
		})
	}
	return records, nil
}

func runCodex(ctx context.Context, runner CodexRunner, arguments ...string) (CLIResult, error) {
	if runner == nil {
		return CLIResult{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex runner is required", nil)
	}
	result, err := runner.Run(ctx, arguments...)
	if err != nil || result.ExitCode != 0 {
		return CLIResult{}, installError("BRIDGE_INSTALL_CODEX_FAILED", "Codex Plugin command failed", nil)
	}
	return result, nil
}

func invalidCodexOutput(message string, cause error) error {
	return installError("BRIDGE_INSTALL_CODEX_INVALID", message, cause)
}
