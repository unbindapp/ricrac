package ops

import "testing"

func TestNoteReferences(t *testing.T) {
	for value, want := range map[string]string{
		"https://x.dev/acme/release-note/spring-launch-0a1b2c3d4e5f/": "0a1b2c3d4e5f",
		"0a1b2c3d4e5f":                         "0a1b2c3d4e5f",
		"53302c2e-69c0-4a7d-ac92-5e7ec601ffd4": "53302c2e-69c0-4a7d-ac92-5e7ec601ffd4",
	} {
		if got := idOrSlug(value); got != want {
			t.Errorf("%s: got %s, want %s", value, got, want)
		}
	}
}
