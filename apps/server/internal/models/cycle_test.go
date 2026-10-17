package models

import "testing"

func TestCycleProgressRoundsToFourDecimalsAsPostgres(t *testing.T) {
	cases := []struct {
		completed, issues int
		want              float64
	}{{0, 0, 0}, {1, 3, 0.3333}, {2, 3, 0.6667}, {1, 8, 0.125}, {1, 20000, 0.0001}, {1, 30000, 0}, {3, 3, 1}}
	for _, c := range cases {
		if got := CycleProgress(c.completed, c.issues); got != c.want {
			t.Errorf("CycleProgress(%d, %d) = %v, want %v", c.completed, c.issues, got, c.want)
		}
	}
}

func TestDocumentURLsStartWithTheTitlesWords(t *testing.T) {
	cases := map[string]string{
		"Launch plan: v2!": "http://x/acme/document/launch-plan-v2-0a1b2c3d4e5f",
		"???":              "http://x/acme/document/0a1b2c3d4e5f",
	}
	for title, want := range cases {
		if got := DocumentURL("http://x", "acme", title, "0a1b2c3d4e5f"); got != want {
			t.Errorf("DocumentURL(%q) = %s, want %s", title, got, want)
		}
	}
}
