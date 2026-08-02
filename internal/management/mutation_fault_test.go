package management

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMutationFaultRollbackPreservesForcedUpdateContent(t *testing.T) {
	points := []mutationFaultPoint{
		{phase: mutationPhaseBackup, moment: mutationBefore, index: 0},
		{phase: mutationPhaseBackup, moment: mutationAfter, index: 0},
		{phase: mutationPhaseTarget, moment: mutationBefore, index: 0},
		{phase: mutationPhaseTarget, moment: mutationAfter, index: 0},
		{phase: mutationPhasePolicy, moment: mutationBefore, index: 0},
		{phase: mutationPhasePolicy, moment: mutationAfter, index: 0},
		{phase: mutationPhaseState, moment: mutationBefore, index: 0},
		{phase: mutationPhaseState, moment: mutationAfter, index: 0},
	}
	for _, point := range points {
		point := point
		t.Run(point.String(), func(t *testing.T) {
			fixture := newPrepareFixture(t)
			installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
			if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotWithoutBackup(t, fixture.root)
			_, err = applyMutationPlan(prepared.plan, func(actual mutationFaultPoint) error {
				if actual == point {
					return fmt.Errorf("injected %s", point.String())
				}
				return nil
			})
			if err == nil {
				t.Fatal("injected update succeeded")
			}
			if after := snapshotWithoutBackup(t, fixture.root); !reflect.DeepEqual(before, after) {
				t.Fatalf("rollback differs at %s:\nbefore=%#v\nafter=%#v", point.String(), before, after)
			}
		})
	}
}

func TestMutationFaultRollbackPreservesForcedUninstallDirectories(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	if err := os.WriteFile(installed.targetActions[0].destination, []byte("drifted cursor\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	prototype, err := PrepareUninstall(fixture.environment, UninstallRequest{Project: project, Targets: "cursor", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	points := []mutationFaultPoint{
		{phase: mutationPhaseTarget, moment: mutationAfter, index: 0},
		{phase: mutationPhasePolicy, moment: mutationAfter, index: 0},
		{phase: mutationPhaseState, moment: mutationAfter, index: 0},
	}
	for index, action := range prototype.plan.directoryActions {
		phase := mutationPhaseTargetDirectory
		if action.namespace {
			phase = mutationPhaseNamespaceDirectory
		}
		points = append(points, mutationFaultPoint{phase: phase, moment: mutationBefore, index: index})
		points = append(points, mutationFaultPoint{phase: phase, moment: mutationAfter, index: index})
	}
	for _, point := range points {
		point := point
		t.Run(point.String(), func(t *testing.T) {
			caseFixture := newPrepareFixture(t)
			caseProject := filepath.Join(caseFixture.root, "project")
			if err := os.Mkdir(caseProject, 0o755); err != nil {
				t.Fatal(err)
			}
			caseInstalled := materializeInstallRequest(t, caseFixture, InstallRequest{Project: caseProject, Targets: "cursor"})
			if err := os.WriteFile(caseInstalled.targetActions[0].destination, []byte("drifted cursor\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			prepared, err := PrepareUninstall(caseFixture.environment, UninstallRequest{Project: caseProject, Targets: "cursor", Force: true})
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotWithoutBackup(t, caseFixture.root)
			_, err = applyMutationPlan(prepared.plan, func(actual mutationFaultPoint) error {
				if actual == point {
					return fmt.Errorf("injected %s", point.String())
				}
				return nil
			})
			if err == nil {
				t.Fatal("injected uninstall succeeded")
			}
			if after := snapshotWithoutBackup(t, caseFixture.root); !reflect.DeepEqual(before, after) {
				t.Fatalf("rollback differs at %s:\nbefore=%#v\nafter=%#v", point.String(), before, after)
			}
		})
	}
}
