package search_service

import (
	"slices"
	"testing"
)

func TestSearchTypes(t *testing.T) {
	if got, _ := parseTypes(nil); !slices.Equal(got, []string{"issue", "project", "document", "initiative"}) {
		t.Errorf("by default got %v", got)
	}
	if got, _ := parseTypes([]string{"Comments", " issue", "comment"}); !slices.Equal(got, []string{"comment", "issue"}) {
		t.Errorf("got %v", got)
	}
	if _, err := parseTypes([]string{"cycles"}); err == nil {
		t.Error("cycles should fail")
	}
}

func TestRanksReadAsPostgresWritesThem(t *testing.T) {
	if got := jsonRank(0.0607927); got != 0.0607927 {
		t.Errorf("got %v", got)
	}
}
