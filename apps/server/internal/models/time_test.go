package models

import (
	"testing"
	"time"
)

func TestTimesAreWrittenAsPostgresWritesThemInJSON(t *testing.T) {
	cases := map[string]time.Time{
		"2026-01-02T03:04:05+00:00":        time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		"2026-01-02T03:04:05.12+00:00":     time.Date(2026, 1, 2, 3, 4, 5, 120_000_000, time.UTC),
		"2026-10-17T01:39:36.949083+00:00": time.Date(2026, 10, 17, 1, 39, 36, 949_083_999, time.UTC),
		"2026-01-02T01:04:05+00:00":        time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 2*3600)),
	}
	for want, at := range cases {
		if got := Time(at); got != want {
			t.Errorf("Time(%v) = %s, want %s", at, got, want)
		}
	}
	if got := Date(time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)); got != "2030-01-02" {
		t.Errorf("Date = %s", got)
	}
}
