package timerange

import "testing"

func TestResolvePeriod(t *testing.T) {
	r, err := Resolve(Options{Period: Period1h})
	if err != nil {
		t.Fatal(err)
	}
	if r.Start == "" || r.End == "" {
		t.Fatal("expected non-empty range")
	}
	if r.Label != "last 1h" {
		t.Fatalf("label = %q, want last 1h", r.Label)
	}
}

func TestResolveHours(t *testing.T) {
	r, err := Resolve(Options{Hours: 3, HoursSet: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Label != "last 3h" {
		t.Fatalf("label = %q, want last 3h", r.Label)
	}
}

func TestParsePeriodInvalid(t *testing.T) {
	if _, err := ParsePeriod("2d"); err == nil {
		t.Fatal("expected error for invalid period")
	}
}
