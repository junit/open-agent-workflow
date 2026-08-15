package checklist

import "testing"

func TestSummarizeCountsOnlyChecklistItems(t *testing.T) {
	input := "# Release\n\n- [x] Policy\n- [ ] Tests\ncontext - [x] is not an item\n- [X] Review\n"
	got := Summarize(input)
	if got.Complete != 2 || got.Total != 3 {
		t.Fatalf("Summarize() = %#v, want complete=2 total=3", got)
	}
}

func TestSummarizeEmptyChecklist(t *testing.T) {
	if got := Summarize("# Notes\nNo checklist yet.\n"); got.Complete != 0 || got.Total != 0 {
		t.Fatalf("Summarize(empty) = %#v, want zero counts", got)
	}
}
