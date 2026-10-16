package ops

import (
	"testing"
	"time"
)

func TestFirstCycleStartsOnTheStartDay(t *testing.T) {
	thursday := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	cases := map[int]string{1: "2026-10-12T00:00:00+00:00", 4: "2026-10-15T00:00:00+00:00", 5: "2026-10-09T00:00:00+00:00"}
	for day, want := range cases {
		if got := chronoRFC3339(firstStart(thursday, day)); got != want {
			t.Errorf("start day %d: got %s, want %s", day, got, want)
		}
	}
}
