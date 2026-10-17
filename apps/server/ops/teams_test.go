package ops

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRotationShifts(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	date := func(month time.Month, day int) time.Time { return time.Date(2026, month, day, 0, 0, 0, 0, time.UTC) }
	start, today := date(1, 5), date(1, 20)
	all := shifts([]uuid.UUID{a, b}, 1, start, today, 3, nil)
	if len(all) != 3 || all[0].person != a || all[1].person != b || all[2].person != a {
		t.Fatalf("people take turns: %v", all)
	}
	if !all[0].starts.Equal(date(1, 19)) {
		t.Fatalf("the current shift starts on the 19th, not %v", all[0].starts)
	}
	mine := shifts([]uuid.UUID{a, b}, 2, start, today, 2, &b)
	if !mine[0].starts.Equal(date(1, 19)) || !mine[1].starts.Equal(date(2, 16)) {
		t.Fatalf("only b's shifts: %v", mine)
	}
}
