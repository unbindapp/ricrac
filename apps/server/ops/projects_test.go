package ops

import "testing"

func TestTrashedSlugTails(t *testing.T) {
	cases := map[string]string{
		"http://x.dev/acme/project/launch-0a1b2c3d4e5f": "0a1b2c3d4e5f",
		"Launch": "Launch",
	}
	for value, want := range cases {
		if got := trashedSlugTail(value); got != want {
			t.Errorf("%s: got %q, want %q", value, got, want)
		}
	}
}
