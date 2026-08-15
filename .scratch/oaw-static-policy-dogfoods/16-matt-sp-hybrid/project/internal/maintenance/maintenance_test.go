package maintenance

import "testing"

func TestEvaluateAcceptsBoundaryTouchAndUnorderedInput(t *testing.T) {
	got, err := Evaluate([]string{"10:00-11:00", "09:00-10:00"})
	if err != nil || got != "valid maintenance plan: 2 windows" {
		t.Fatalf("Evaluate() = %q, %v", got, err)
	}
}

func TestEvaluateNamesOverlap(t *testing.T) {
	_, err := Evaluate([]string{"09:00-10:00", "09:30-11:00"})
	if err == nil || err.Error() != `overlap: "09:00-10:00" and "09:30-11:00"` {
		t.Fatalf("Evaluate() error = %v", err)
	}
}

func TestEvaluateRejectsMalformedAndBackwardsWindows(t *testing.T) {
	for _, input := range []string{"9:00-10:00", "10:00-09:00", "23:61-23:62"} {
		if _, err := Evaluate([]string{input}); err == nil {
			t.Errorf("Evaluate(%q) accepted invalid input", input)
		}
	}
}

func TestEvaluateAcceptsEmptyPlan(t *testing.T) {
	got, err := Evaluate(nil)
	if err != nil || got != "valid maintenance plan: 0 windows" {
		t.Fatalf("Evaluate(nil) = %q, %v", got, err)
	}
}
