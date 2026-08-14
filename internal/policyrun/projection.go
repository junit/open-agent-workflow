package policyrun

import (
	"fmt"
	"reflect"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

var policySlots = []string{
	"problem-framing", "solution-specification", "delivery-planning", "workspace-preparation",
	"implementation", "implementation-tdd", "incident-recovery", "review-remediation",
	"fresh-verification", "closeout",
}

// ProjectResponsibilities creates the human-readable Plan projection from the
// reducer's authority-neutral Policy Offer. It does not consult machine facts.
func ProjectResponsibilities(profileID policyflow.ProfileID, offer policyflow.Offer) (ResponsibilityMap, error) {
	result := ResponsibilityMap{}
	for _, slot := range policySlots {
		result[slot] = []Responsibility{}
	}
	for _, profile := range offer.Profiles {
		if profile.ID != profileID {
			continue
		}
		for _, route := range profile.Routes {
			kind := string(route.Kind)
			if route.Credited {
				kind = "credited-skill"
			}
			mode := string(route.Mode)
			if !route.Available {
				mode = "unavailable"
			}
			for _, slot := range route.Covers {
				key := string(slot)
				if _, known := result[key]; !known {
					return nil, fmt.Errorf("POLICY_RUN_INVALID: unknown lifecycle slot %q", slot)
				}
				result[key] = append(result[key], Responsibility{Route: route.Name, Mode: mode, Kind: kind})
			}
		}
		for _, incident := range profile.IncidentRoutes {
			mode := string(incident.Mode)
			route := incident.Skill
			if !incident.Available {
				mode = "conditional-unavailable"
				route = "unhandled:" + string(incident.Incident)
			}
			result["incident-recovery"] = append(result["incident-recovery"], Responsibility{
				Route: route, Mode: mode, Kind: "incident-handler",
			})
		}
		for _, slot := range policySlots {
			if len(result[slot]) == 0 {
				return nil, fmt.Errorf("POLICY_RUN_INVALID: Profile %s has no projected responsibility for %s", profileID, slot)
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("POLICY_RUN_INVALID: unknown Profile %s", profileID)
}

func expectedResponsibilities(inventory policyflow.RouteInventory, profileID policyflow.ProfileID) (ResponsibilityMap, error) {
	offer, err := policyflow.New().Offer(inventory)
	if err != nil {
		return nil, err
	}
	return ProjectResponsibilities(profileID, offer)
}

func equalResponsibilities(left, right ResponsibilityMap) bool {
	return reflect.DeepEqual(left, right)
}
