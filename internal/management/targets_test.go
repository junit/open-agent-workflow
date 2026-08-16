package management

import (
	"reflect"
	"testing"
)

func TestTargetRegistryProvidesOrderedNativeArtifacts(t *testing.T) {
	wantIDs := []string{"claude", "codex", "gemini", "opencode", "cursor", "windsurf", "cline", "roo", "copilot"}
	gotIDs := make([]string, 0, len(targetRegistry))
	for _, candidate := range targetRegistry {
		gotIDs = append(gotIDs, candidate.ID)
		wantArtifactCount := 2
		if candidate.ID == "codex" {
			wantArtifactCount = 3
		}
		if len(candidate.Artifacts) != wantArtifactCount {
			t.Fatalf("target %q has %d artifacts, want %d", candidate.ID, len(candidate.Artifacts), wantArtifactCount)
		}
		if candidate.Artifacts[0].ID != routerArtifactID || candidate.Artifacts[0].Kind != activationRouterArtifactKind {
			t.Fatalf("target %q first artifact = %#v, want activation router", candidate.ID, candidate.Artifacts[0])
		}
		wantRouterOwnership := "managed-block"
		switch candidate.ID {
		case "cursor", "windsurf", "cline", "roo", "copilot":
			wantRouterOwnership = "owned-file"
		}
		if candidate.Artifacts[0].Ownership != wantRouterOwnership {
			t.Fatalf("target %q router ownership = %q, want %q", candidate.ID, candidate.Artifacts[0].Ownership, wantRouterOwnership)
		}
		seen := make(map[string]struct{}, len(candidate.Artifacts))
		for _, artifact := range candidate.Artifacts {
			if artifact.ID == "" {
				t.Fatalf("target %q has an empty artifact ID", candidate.ID)
			}
			if _, exists := seen[artifact.ID]; exists {
				t.Fatalf("target %q repeats artifact %q", candidate.ID, artifact.ID)
			}
			seen[artifact.ID] = struct{}{}
			if artifact.ProjectSuffix == "" || artifact.Ownership == "" {
				t.Fatalf("target %q has incomplete project artifact %#v", candidate.ID, artifact)
			}
			if artifact.ID != routerArtifactID && artifact.Ownership != "owned-file" {
				t.Fatalf("target %q native artifact %q ownership = %q", candidate.ID, artifact.ID, artifact.Ownership)
			}
		}
		if _, ok := findTargetArtifact(candidate.ID, nativeEntrypointArtifactID); !ok {
			t.Fatalf("target %q has no native-entrypoint artifact", candidate.ID)
		}
		if targetArtifactPosition(candidate.ID, routerArtifactID) == 0 || targetArtifactPosition(candidate.ID, nativeEntrypointArtifactID) == 0 {
			t.Fatalf("target %q has no stable artifact positions", candidate.ID)
		}
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("target IDs = %v, want %v", gotIDs, wantIDs)
	}
	if got := targetArtifactPosition("codex", nativePolicyArtifactID); got == 0 {
		t.Fatal("Codex native-policy artifact has no registry position")
	}
}

func TestTargetScopeMatrixRemainsFourUserNineProject(t *testing.T) {
	user, err := normalizeTargets("", "user")
	if err != nil {
		t.Fatal(err)
	}
	project, err := normalizeTargets("", "project")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"claude", "codex", "gemini", "opencode"}; !reflect.DeepEqual(user, want) {
		t.Fatalf("user targets = %v, want %v", user, want)
	}
	if want := []string{"claude", "codex", "gemini", "opencode", "cursor", "windsurf", "cline", "roo", "copilot"}; !reflect.DeepEqual(project, want) {
		t.Fatalf("project targets = %v, want %v", project, want)
	}
	if _, err := normalizeTargets("cursor", "user"); err == nil {
		t.Fatal("user-only normalization accepted project-only target")
	}
}

func TestTargetArtifactsForScopeCopiesAndRejectsUnknownOrUnsupported(t *testing.T) {
	artifacts, err := targetArtifactsForScope("codex", "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 3 || artifacts[0].ID != routerArtifactID || artifacts[1].ID != nativeEntrypointArtifactID || artifacts[2].ID != nativePolicyArtifactID {
		t.Fatalf("Codex user artifacts = %#v", artifacts)
	}
	artifacts[0].UserSuffix = "mutated"
	fresh, ok := findTargetArtifact("codex", routerArtifactID)
	if !ok || fresh.UserSuffix == "mutated" {
		t.Fatal("targetArtifactsForScope returned registry-backed storage")
	}
	if _, err := targetArtifactsForScope("cursor", "user"); err == nil {
		t.Fatal("targetArtifactsForScope accepted unsupported user scope")
	}
	if _, err := targetArtifactsForScope("missing", "project"); err == nil {
		t.Fatal("targetArtifactsForScope accepted unknown target")
	}
}
