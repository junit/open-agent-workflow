//go:build policyroute_real_home

package policyroute_test

import (
	"os"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyroute"
)

func TestRealCodexHomeMakesEveryBuiltInProfileRoutable(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	offer, err := policyflow.New().Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []policyflow.ProfileID{
		policyflow.ProfileSPFull,
		policyflow.ProfileMattFull,
		policyflow.ProfileECCFull,
		policyflow.ProfileMattSPHybrid,
	} {
		profile := requireProfile(t, offer, id)
		if !profile.HostRoutable {
			t.Errorf("%s missing routes = %#v", id, profile.Missing)
		}
	}
}
