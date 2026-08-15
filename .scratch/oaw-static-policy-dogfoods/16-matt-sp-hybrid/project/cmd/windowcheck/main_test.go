package main

import (
	"bytes"
	"testing"
)

func TestRunReportsValidPlan(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"09:00-10:00", "10:00-11:00"}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if stdout.String() != "valid maintenance plan: 2 windows\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunReportsDomainError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"09:00-10:00", "09:30-11:00"}, &stdout, &stderr); status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if stdout.Len() != 0 || stderr.String() != "overlap: \"09:00-10:00\" and \"09:30-11:00\"\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunRequiresAWindow(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run(nil, &stdout, &stderr); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if stdout.Len() != 0 || stderr.String() != "usage: windowcheck <window> [window...]\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}
