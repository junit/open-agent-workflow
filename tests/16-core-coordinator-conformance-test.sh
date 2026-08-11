#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
CONFORMANCE_TEMP=
FIXTURE_HELPER_ROOT=

cleanup() {
  if [ -n "$CONFORMANCE_TEMP" ] && [ -d "$CONFORMANCE_TEMP" ]; then
    rm -rf -- "$CONFORMANCE_TEMP"
  fi
  if [ -n "$FIXTURE_HELPER_ROOT" ] && [ -d "$FIXTURE_HELPER_ROOT" ]; then
    rm -rf -- "$FIXTURE_HELPER_ROOT"
  fi
}

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

run_oaw() {
  name=$1
  expected_status=$2
  input=$3
  shift 3
  set +e
  HOME="$CONFORMANCE_TEMP/home" \
    XDG_CONFIG_HOME="$CONFORMANCE_TEMP/config" \
    XDG_STATE_HOME="$CONFORMANCE_TEMP/state" \
    PATH="$CONFORMANCE_TEMP/traps:$PATH" \
    OAW_MODEL_SENTINEL="$CONFORMANCE_TEMP/model-executed" \
    "$CONFORMANCE_TEMP/release/oaw" "$@" \
    <"$input" >"$CONFORMANCE_TEMP/$name.stdout" 2>"$CONFORMANCE_TEMP/$name.stderr"
  status=$?
  set -e
  if [ "$status" -ne "$expected_status" ]; then
    fail "$name exited $status, want $expected_status: $(cat "$CONFORMANCE_TEMP/$name.stderr")"
  fi
}

assert_no_workflow_state() {
  [ ! -e "$CONFORMANCE_TEMP/state/open-agent-workflow/workflows" ] ||
    fail "non-Workflow command created Workflow State"
}

assert_rejected_result() {
  output=$1
  grep -F '"kind":"REJECTED"' "$output" >/dev/null ||
    fail "Workflow rejection is not a canonical REJECTED Result: $(cat "$output")"
}

write_fixture_helper() {
  FIXTURE_HELPER_ROOT=$(mktemp -d "$REPOSITORY/.oaw-conformance-fixture.XXXXXX") ||
    fail "cannot create fixture helper directory"
  cat >"$FIXTURE_HELPER_ROOT/main.go" <<'EOF'
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: fixture-helper output-directory")
		os.Exit(2)
	}
	if err := generate(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(output string) error {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		return err
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-current-conformance",
		Topology: execution.TopologyCurrent, Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		return err
	}
	inventory, err := host.BuildBindingInventoryV3("codex", nil)
	if err != nil {
		return err
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "local/conformance-host",
		IntegrationVersion: "3.0.0", SessionID: "session-current-conformance", ManifestDigest: manifest.Digest,
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, FeatureObservations: []host.FeatureObservation{},
		HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		return err
	}
	graphSelection := profile.Selection{
		Profile: "SP-FULL", RecipeID: "oaw/delivery", RecipeDigest: strings.Repeat("c", 64),
		Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{},
	}
	graphSelectionDigest, _, err := canonicaljson.Digest(graphSelection)
	if err != nil {
		return err
	}
	const startKey = "conformance-workflow-start"
	start := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: startKey,
		Start: &coordinator.StartInput{
			RequestID: "request-conformance", DeliverableID: "deliverable-conformance", InputDigest: strings.Repeat("b", 64),
			Proposal: classification.ClassificationProposal{
				SchemaVersion: classification.ProposalSchemaV1, Traits: []classification.TraitObservation{},
				Resources: []classification.Resource{}, Evidence: []classification.ProposalEvidence{},
			},
			Selection: core.Selection{
				Profile: "SP-FULL", RecipeID: graphSelection.RecipeID, RecipeDigest: graphSelection.RecipeDigest,
				ProfileSource: core.SelectionUser, Topology: execution.TopologyCurrent,
				TopologySource: core.SelectionHostOnlyOption, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{},
				Overlays: []string{}, GraphSelectionDigest: graphSelectionDigest,
			},
			HostSession: session, Environment: environment,
		},
	}
	workflowHash := sha256.Sum256([]byte(startKey))
	workflowID := "workflow-" + hex.EncodeToString(workflowHash[:16])
	cursor, err := execution.NewGraphCursor("requirements-alignment", execution.CursorBinding, "superpowers-brainstorming", 1)
	if err != nil {
		return err
	}
	prepare := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
		MessageID: "message-prepare", IdempotencyKey: "conformance-workflow-prepare",
		WorkflowID: workflowID, ExpectedRevision: 1,
		Prepare: &coordinator.PrepareInput{
			RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"},
			TerminationCondition: "return conformance evidence", InputReferences: []coordinator.ArtifactReference{},
			EvidenceRequirements: []coordinator.EvidenceRequirement{},
		},
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: host.ReceiptStarted,
		WorkflowID: workflowID, BundleID: "bundle-0123456789abcdef0123456789abcdef",
		BundleGeneration: 1, BundleDigest: strings.Repeat("c", 64), Cursor: cursor,
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, DispatchDigest: strings.Repeat("d", 64),
		ContextFreshness: host.ContextShared, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		return err
	}
	receiptCommand := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandReceipt,
		MessageID: "message-receipt", IdempotencyKey: "conformance-workflow-receipt",
		WorkflowID: workflowID, ExpectedRevision: 2, Receipt: &coordinator.ReceiptInput{Receipt: receipt},
	}
	for name, command := range map[string]coordinator.Command{
		"start.json": start, "prepare.json": prepare, "receipt.json": receiptCommand,
	} {
		raw, marshalErr := canonicaljson.Marshal(command)
		if marshalErr != nil {
			return marshalErr
		}
		if writeErr := os.WriteFile(filepath.Join(output, name), raw, 0o600); writeErr != nil {
			return writeErr
		}
	}
	const oldWorkflowID = "workflow-0123456789abcdef0123456789abcdef"
	head := struct {
		SchemaVersion  string `json:"schema_version"`
		WorkflowID     string `json:"workflow_id"`
		Revision       uint64 `json:"revision"`
		RevisionDigest string `json:"revision_digest"`
		Digest         string `json:"digest"`
	}{"oaw.workflow-head/v1", oldWorkflowID, 1, strings.Repeat("e", 64), ""}
	head.Digest, _, err = canonicaljson.Digest(head)
	if err != nil {
		return err
	}
	for name, value := range map[string]any{
		"old-head.json": head,
		"inspect-old.json": coordinator.Command{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandInspect, WorkflowID: oldWorkflowID,
		},
	} {
		raw, marshalErr := canonicaljson.Marshal(value)
		if marshalErr != nil {
			return marshalErr
		}
		if writeErr := os.WriteFile(filepath.Join(output, name), raw, 0o600); writeErr != nil {
			return writeErr
		}
	}
	return nil
}
EOF
}

trap cleanup EXIT HUP INT TERM

CONFORMANCE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-core-conformance.XXXXXX") ||
  fail "cannot create conformance directory"
mkdir -p "$CONFORMANCE_TEMP/release" "$CONFORMANCE_TEMP/home" "$CONFORMANCE_TEMP/config" \
  "$CONFORMANCE_TEMP/state" "$CONFORMANCE_TEMP/traps" "$CONFORMANCE_TEMP/fixtures"

(cd "$REPOSITORY" && go build -o "$CONFORMANCE_TEMP/release/oaw" ./cmd/oaw) ||
  fail "cannot build oaw"

for model_command in codex claude gemini opencode; do
  {
    printf '%s\n' '#!/usr/bin/env bash'
    printf '%s\n' 'printf "%s\n" "$0" >>"$OAW_MODEL_SENTINEL"'
    printf '%s\n' 'exit 99'
  } >"$CONFORMANCE_TEMP/traps/$model_command"
  chmod 755 "$CONFORMANCE_TEMP/traps/$model_command"
done

run_oaw help 0 /dev/null --help
run_oaw catalog 0 /dev/null catalog validate
run_oaw providers 0 /dev/null providers inspect --host codex --format json
run_oaw removed-runtime 64 /dev/null runtime exchange
run_oaw removed-run 64 /dev/null run --host codex
assert_no_workflow_state

write_fixture_helper
(cd "$REPOSITORY" && go run "$FIXTURE_HELPER_ROOT/main.go" "$CONFORMANCE_TEMP/fixtures") ||
  fail "cannot generate canonical Workflow fixtures"

for command in start prepare receipt; do
  run_oaw "workflow-$command" 65 "$CONFORMANCE_TEMP/fixtures/$command.json" workflow exchange
  assert_rejected_result "$CONFORMANCE_TEMP/workflow-$command.stdout"
done

old_workflow=workflow-0123456789abcdef0123456789abcdef
old_workflow_root=$CONFORMANCE_TEMP/state/open-agent-workflow/workflows/records/$old_workflow
mkdir -p "$old_workflow_root/revisions"
cp "$REPOSITORY/internal/integration/testdata/core-coordinator/old-v1-revision.json" \
  "$old_workflow_root/revisions/00000000000000000001.json"
cp "$CONFORMANCE_TEMP/fixtures/old-head.json" "$old_workflow_root/HEAD"
run_oaw workflow-old-state 65 "$CONFORMANCE_TEMP/fixtures/inspect-old.json" workflow exchange
assert_rejected_result "$CONFORMANCE_TEMP/workflow-old-state.stdout"
grep -F 'WORKFLOW_STATE_UNSUPPORTED' "$CONFORMANCE_TEMP/workflow-old-state.stdout" >/dev/null ||
  fail "old Workflow Revision v1 did not fail closed: $(cat "$CONFORMANCE_TEMP/workflow-old-state.stdout")"
cmp "$REPOSITORY/internal/integration/testdata/core-coordinator/old-v1-revision.json" \
  "$old_workflow_root/revisions/00000000000000000001.json" >/dev/null ||
  fail 'old Workflow Revision v1 fixture was modified'

workflow_state=$CONFORMANCE_TEMP/state/open-agent-workflow/workflows
[ -d "$workflow_state/records" ] || fail "Workflow exchange did not initialize Workflow State"
[ -n "$(find "$workflow_state" -type f -print -quit)" ] || fail "Workflow exchange created no Workflow record"
[ ! -e "$CONFORMANCE_TEMP/model-executed" ] ||
  fail "OAW launched a model process: $(cat "$CONFORMANCE_TEMP/model-executed")"

printf 'PASS: Core Coordinator black box owns state but launches no model process\n'
