package rollout

import (
	"reflect"
	"testing"
)

func TestBucketUsesStableFNV1aContract(t *testing.T) {
	if got := Bucket("hello"); got != 23 {
		t.Fatalf("Bucket(hello) = %d, want 23", got)
	}
}

func TestSelectPreservesOrderAtBoundary(t *testing.T) {
	got, err := Select(50, []string{"a", "b", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Select() = %q, want %q", got, want)
	}
}

func TestSelectHandlesZeroAndHundredPercent(t *testing.T) {
	keys := []string{"a", "b", "a"}
	got, err := Select(0, keys)
	if err != nil || len(got) != 0 {
		t.Fatalf("Select(0) = %q, %v; want empty", got, err)
	}
	got, err = Select(100, keys)
	if err != nil || !reflect.DeepEqual(got, keys) {
		t.Fatalf("Select(100) = %q, %v; want %q", got, err, keys)
	}
}

func TestSelectRejectsInvalidPercentages(t *testing.T) {
	for _, percentage := range []int{-1, 101} {
		if _, err := Select(percentage, []string{"a"}); err == nil {
			t.Errorf("Select(%d) succeeded", percentage)
		}
	}
}

func TestSelectRejectsEmptyKeys(t *testing.T) {
	if _, err := Select(50, []string{"valid", ""}); err == nil {
		t.Fatal("Select() accepted an empty key")
	}
}
