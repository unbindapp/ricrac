package ops

import (
	"slices"
	"strings"
	"testing"
)

func TestURLsMustBeHTTPSExceptOnThisMachine(t *testing.T) {
	for _, value := range []string{"https://hooks.example.com/ricrac", "http://localhost:8080/hook", "http://127.0.0.1:9/x"} {
		if _, err := checkWebhookURL(value); err != nil {
			t.Errorf("%s: %v", value, err)
		}
	}
	for _, value := range []string{"http://hooks.example.com", "ftp://localhost", "not a url"} {
		if _, err := checkWebhookURL(value); err == nil {
			t.Errorf("%s should be refused", value)
		}
	}
}

func TestResourceTypesAreCanonical(t *testing.T) {
	types, err := checkResourceTypes([]string{"issue", "CUSTOMERNEED", "Issue"})
	if err != nil || !slices.Equal(types, []string{"Issue", "CustomerNeed"}) {
		t.Errorf("got %v, %v", types, err)
	}
	if _, err := checkResourceTypes([]string{"Issues"}); err == nil || !strings.Contains(err.Error(), "ReleaseNote") {
		t.Errorf("the error lists the types: %v", err)
	}
}
